package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"incus-web/config"
	"incus-web/event"
	"incus-web/handler"
	"incus-web/incus"
	"incus-web/middleware"
	"incus-web/service"

	"github.com/gin-gonic/gin"
)

// Version 由构建脚本通过 -ldflags "-X main.Version=..." 注入（build.ps1 已在使用）
var Version = "dev"

func main() {
	config.Init()
	event.Init()

	log.Printf("IncusPanel %s 启动中（数据目录: %s）", Version, config.DataDir)

	// 创建 Incus 客户端
	client := incus.NewClient()

	// 启动 Incus 事件监听
	incus.StartEventClient(event.GlobalBroker)

	// 初始化服务层
	instanceSvc := service.NewInstanceService(client, event.GlobalBroker)
	imageSvc := service.NewImageService(client)
	storageSvc := service.NewStorageService(client)
	networkSvc := service.NewNetworkService(client)
	profileSvc := service.NewProfileService(client)

	// 远程镜像索引缓存：落盘 + 后台定期刷新，保证第二次打开搜索框秒开
	imgCache := handler.NewRemoteImageCache()
	if err := imgCache.Load(); err != nil {
		log.Printf("远程镜像缓存加载跳过（%v），将重新构建", err)
	}
	stopCacheRefresh := imgCache.StartBackground(time.Duration(config.RemoteImageRefreshInterval) * time.Minute)
	defer stopCacheRefresh()

	// 初始化处理器层
	serverH := handler.NewServerHandler(client)
	installH := handler.NewInstallHandler()
	instanceH := handler.NewInstanceHandler(instanceSvc)
	imageH := handler.NewImageHandler(imageSvc)
	storageH := handler.NewStorageHandler(storageSvc)
	networkH := handler.NewNetworkHandler(networkSvc)
	profileH := handler.NewProfileHandler(profileSvc)
	snapshotH := handler.NewSnapshotHandler(client)
	backupH := handler.NewBackupHandler(client)
	terminalH := handler.NewTerminalHandler()
	eventH := handler.NewEventHandler(event.GlobalBroker, client)
	fileH := handler.NewFileHandler(client)
	opH := handler.NewOperationHandler(client)
	netDiagH := handler.NewNetworkDiagHandler(client, imgCache)

	// 设置 Gin
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	// 自定义访问日志：只记录路径，不记录 query。
	// 终端 / 备份下载 / SSE 都把 token 放在 ?token= 里，默认日志会把令牌明文写进日志文件。
	// 注意：gin 的 LogFormatterParams.Path 已拼上 RawQuery，必须手动截断。
	r.Use(gin.LoggerWithFormatter(func(p gin.LogFormatterParams) string {
		path := p.Path
		if i := strings.IndexByte(path, '?'); i >= 0 {
			path = path[:i]
		}
		return fmt.Sprintf("[GIN] %s | %3d | %13v | %15s | %-7s %s\n",
			p.TimeStamp.Format("2006/01/02 - 15:04:05"),
			p.StatusCode, p.Latency, p.ClientIP, p.Method, path)
	}))
	r.Use(gin.Recovery())

	// Web 管理界面（go:embed 前端构建产物，SPA 回退）
	r.GET("/", handler.ServeUI)
	r.NoRoute(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/api/") {
			c.JSON(http.StatusNotFound, gin.H{"code": 404, "error": "no such api endpoint"})
			return
		}
		handler.ServeUI(c)
	})

	// 健康检查（无需认证）
	r.GET("/api/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	// 需要认证的路由组
	api := r.Group("/api")
	api.Use(middleware.Auth())
	{
		// 11. 服务器信息
		api.GET("/server/info", serverH.GetServerInfo)
		api.GET("/server/resources", serverH.GetServerResources)
		api.GET("/server/features", serverH.GetServerFeatures)

		// 15. Incus 安装向导
		api.GET("/server/install-status", installH.InstallStatus)
		api.POST("/server/install", installH.InstallIncus)
		api.GET("/server/install-log", installH.InstallLog)

		// 1. 实例管理
		api.GET("/instances", instanceH.ListInstances)
		api.GET("/instances/:name", instanceH.GetInstance)
		api.POST("/instances", instanceH.CreateInstance)
		api.DELETE("/instances/:name", instanceH.DeleteInstance)
		api.PUT("/instances/:name/state", instanceH.ChangeInstanceState)
		api.PATCH("/instances/:name/config", instanceH.UpdateInstanceConfig)

		// 2. 镜像管理
		api.GET("/images", imageH.ListImages)
		api.POST("/images/refresh", imageH.RefreshImages)
		api.POST("/images/import", imageH.ImportImage)
		api.POST("/images/upload", imageH.UploadImage)
		api.DELETE("/images/:fingerprint", imageH.DeleteImage)
		api.POST("/images/export", imageH.ExportImage)
		// 前端通过 <a download> 触发原生下载（带 ?token=），必须提供 GET 流式接口
		api.GET("/images/:fingerprint/export", imageH.ExportImageStream)

		// 3. 快照管理
		api.GET("/instances/:name/snapshots", snapshotH.ListSnapshots)
		api.POST("/instances/:name/snapshots", snapshotH.CreateSnapshot)
		api.POST("/instances/:name/snapshots/:snap/restore", snapshotH.RestoreSnapshot)
		api.POST("/instances/:name/snapshots/:snap/rename", snapshotH.RenameSnapshot)
		api.DELETE("/instances/:name/snapshots/:snap", snapshotH.DeleteSnapshot)

		// 4. 备份与导出
		api.POST("/instances/:name/backup", backupH.CreateBackup)
		api.GET("/instances/:name/backups", backupH.ListBackups)
		api.GET("/instances/:name/backups/:file/download", backupH.DownloadBackup)
		api.DELETE("/instances/:name/backups/:file", backupH.DeleteBackup)
		api.POST("/instances/:name/export", backupH.ExportInstance)

		// 5. 存储池管理
		api.GET("/storage-pools", storageH.ListStoragePools)
		api.GET("/storage-pools/:name", storageH.GetStoragePool)
		api.POST("/storage-pools", storageH.CreateStoragePool)
		api.DELETE("/storage-pools/:name", storageH.DeleteStoragePool)
		api.GET("/storage-pools/:name/volumes", storageH.ListStorageVolumes)
		api.POST("/storage-pools/:name/volumes", storageH.CreateStorageVolume)

		// 6. 网络管理
		api.GET("/networks", networkH.ListNetworks)
		api.GET("/networks/:name", networkH.GetNetwork)
		api.POST("/networks", networkH.CreateNetwork)
		api.PUT("/networks/:name", networkH.UpdateNetwork)
		api.DELETE("/networks/:name", networkH.DeleteNetwork)

		// 7. 配置文件管理
		api.GET("/profiles", profileH.ListProfiles)
		api.GET("/profiles/:name", profileH.GetProfile)
		api.POST("/profiles", profileH.CreateProfile)
		api.PUT("/profiles/:name", profileH.UpdateProfile)
		api.DELETE("/profiles/:name", profileH.DeleteProfile)

		// 8. WebSocket 终端
		api.GET("/instances/:name/terminal", terminalH.HandleTerminal)

		// 9. 文件管理
		api.GET("/instances/:name/files", fileH.ListFiles)
		api.POST("/instances/:name/files/upload", fileH.UploadFile)
		api.GET("/instances/:name/files/download", fileH.DownloadFile)
		api.DELETE("/instances/:name/files", fileH.DeleteFile)

		// 10. 日志与事件
		api.GET("/instances/:name/logs", eventH.GetLogs)
		api.GET("/instances/:name/logs/stream", eventH.StreamLogs)
		api.GET("/events", eventH.HandleGlobalEvents)

		// 12. 操作跟踪
		api.GET("/operations/:id", opH.GetOperation)
		// 13. 网络诊断与修复
		api.GET("/network/diagnose", netDiagH.Diagnose)
		api.POST("/network/repair", netDiagH.Repair)

		// 14. 镜像远程源管理与远程搜索
		netDiagH.RegisterImageRemoteRoutes(api)
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	if config.APITokenGenerated {
		log.Printf("未设置 API_TOKEN，已生成随机令牌：%s", config.APIToken)
		log.Printf("该令牌仅本次运行有效；请用环境变量 API_TOKEN 固定它，否则重启后需要重新登录")
	} else if config.UsesLegacyDefaultToken() {
		log.Printf("警告：正在使用公开的默认 API_TOKEN，任何知道该值的人都能完全控制宿主机上的 Incus，请立即更换")
	}

	addr := net.JoinHostPort(config.BindAddr, port)
	log.Printf("Incus Web server starting on %s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
