package incus

import (
	"incus-web/model"
)

// IncusOperationDetailResponse 操作详情响应
type IncusOperationDetailResponse struct {
	Type       string `json:"type"`
	Status     string `json:"status"`
	StatusCode int    `json:"status_code"`
	Metadata   struct {
		ID          string                 `json:"id"`
		Status      string                 `json:"status"`
		StatusCode  int                    `json:"status_code"`
		Description string                 `json:"description"`
		CreatedAt   string                 `json:"created_at"`
		UpdatedAt   string                 `json:"updated_at"`
		MayCancel   bool                   `json:"may_cancel"`
		Err         string                 `json:"err"`
		Metadata    map[string]interface{} `json:"metadata"`
		Resources   map[string][]string    `json:"resources"`
	} `json:"metadata"`
}

// GetOperation 查询 Incus 操作状态
func (c *Client) GetOperation(id string) (*model.OperationStatus, error) {
	var resp IncusOperationDetailResponse
	// 移除 /1.0 前缀（因为 Get 方法会自动添加）
	cleanID := id
	if len(id) > 5 && id[:5] == "/1.0/" {
		cleanID = id[4:]
	} else if len(id) > 0 && id[0] != '/' {
		cleanID = "/operations/" + id
	}

	if err := c.Get(cleanID, &resp); err != nil {
		return nil, err
	}

	return &model.OperationStatus{
		ID:          resp.Metadata.ID,
		Status:      resp.Metadata.Status,
		StatusCode:  resp.Metadata.StatusCode,
		Description: resp.Metadata.Description,
		CreatedAt:   resp.Metadata.CreatedAt,
		UpdatedAt:   resp.Metadata.UpdatedAt,
		MayCancel:   resp.Metadata.MayCancel,
		Err:         resp.Metadata.Err,
		Metadata:    resp.Metadata.Metadata,
		Resources:   resp.Metadata.Resources,
	}, nil
}

// GetServerInfo 获取服务器基本信息
func (c *Client) GetServerInfo() (*model.ServerInfo, error) {
	type incusServerInfo struct {
		Type       string `json:"type"`
		Status     string `json:"status"`
		StatusCode int    `json:"status_code"`
		Metadata   struct {
			APIStatus   string `json:"api_status"`
			APIVersion  string `json:"api_version"`
			Environment struct {
				Kernel             string `json:"kernel"`
				KernelArchitecture string `json:"kernel_architecture"`
				KernelVersion      string `json:"kernel_version"`
				OSName             string `json:"os_name"`
				OSVersion          string `json:"os_version"`
				Project            string `json:"project"`
				Server             string `json:"server"`
				ServerVersion      string `json:"server_version"`
				Driver             string `json:"driver"`
				LTS                bool   `json:"lts"`
			} `json:"environment"`
			Config map[string]interface{} `json:"config"`
		} `json:"metadata"`
	}

	var resp incusServerInfo
	if err := c.Get("", &resp); err != nil {
		return nil, err
	}

	return &model.ServerInfo{
		APIStatus:  resp.Metadata.APIStatus,
		APIVersion: resp.Metadata.APIVersion,
		Environment: model.ServerEnvironment{
			Kernel:             resp.Metadata.Environment.Kernel,
			KernelArchitecture: resp.Metadata.Environment.KernelArchitecture,
			KernelVersion:      resp.Metadata.Environment.KernelVersion,
			OSName:             resp.Metadata.Environment.OSName,
			OSVersion:          resp.Metadata.Environment.OSVersion,
			Project:            resp.Metadata.Environment.Project,
			Server:             resp.Metadata.Environment.Server,
			ServerVersion:      resp.Metadata.Environment.ServerVersion,
			Driver:             resp.Metadata.Environment.Driver,
			LTS:                resp.Metadata.Environment.LTS,
		},
		Config: resp.Metadata.Config,
	}, nil
}

// GetServerResources 获取服务器资源使用情况
// Incus /1.0/resources 的字段与前端模型不同，此处做归一化：
//   - cpu.total_cores ← cpu.total（总逻辑核数）
//   - cpu.usage       ← /proc/stat 累计 busy ns（Linux；差分可得占用率）
//   - disk.total/used ← 根文件系统 statfs；失败时回退 Incus 磁盘容量之和
func (c *Client) GetServerResources() (*model.ServerResources, error) {
	type incusThread struct {
		ID   int   `json:"id"`
		Idle int64 `json:"idle"`
	}
	type incusCore struct {
		Core    int           `json:"core"`
		Threads []incusThread `json:"threads"`
	}
	type incusSocket struct {
		Socket int         `json:"socket"`
		Cores  []incusCore `json:"cores"`
	}
	type incusResourceResp struct {
		Type       string `json:"type"`
		Status     string `json:"status"`
		StatusCode int    `json:"status_code"`
		Metadata   struct {
			CPU struct {
				Architecture string        `json:"architecture"`
				Total        int           `json:"total"`
				Sockets      []incusSocket `json:"sockets"`
			} `json:"cpu"`
			Memory struct {
				Total int64 `json:"total"`
				Used  int64 `json:"used"`
			} `json:"memory"`
			Storage struct {
				Disks []struct {
					ID   string `json:"id"`
					Type string `json:"type"`
					Size uint64 `json:"size"`
				} `json:"disks"`
			} `json:"storage"`
			GPU struct {
				Cards []struct {
					Driver string `json:"driver"`
				} `json:"cards"`
			} `json:"gpu"`
			Network struct {
				Cards []struct {
					Ports []struct {
						ID string `json:"id"`
					} `json:"ports"`
				} `json:"cards"`
			} `json:"network"`
		} `json:"metadata"`
	}

	var resp incusResourceResp
	if err := c.Get("/resources", &resp); err != nil {
		return nil, err
	}

	res := &model.ServerResources{
		CPU: model.ServerCPUInfo{
			Architecture: resp.Metadata.CPU.Architecture,
			SocketCount:  len(resp.Metadata.CPU.Sockets),
			TotalCores:   resp.Metadata.CPU.Total,
			Usage:        procStatBusyNs(),
		},
		Memory: model.ServerMemoryInfo{
			Total: resp.Metadata.Memory.Total,
			Used:  resp.Metadata.Memory.Used,
		},
		GPU:     &model.ServerGPUInfo{Count: len(resp.Metadata.GPU.Cards)},
		Network: &model.ServerNetInfo{InterfaceCount: 0},
	}
	for _, s := range resp.Metadata.CPU.Sockets {
		res.CPU.SocketCount = len(resp.Metadata.CPU.Sockets)
		if res.CPU.TotalCores == 0 {
			res.CPU.TotalCores += len(s.Cores)
		}
	}
	for _, d := range resp.Metadata.Storage.Disks {
		res.Disk.Total += int64(d.Size)
	}
	for _, card := range resp.Metadata.Network.Cards {
		res.Network.InterfaceCount += len(card.Ports)
	}
	if total, used, ok := statfsRoot(); ok {
		res.Disk.Total = total
		res.Disk.Used = used
	}
	return res, nil
}
