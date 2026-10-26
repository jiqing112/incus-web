<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { api } from '$lib/api';
  import { navigate } from '$lib/router.svelte';
  import { installState } from '$lib/stores/install.svelte';

  // ---------- 状态 ----------
  let step = $state(1);
  let status = $state<any>(null);
  let checking = $state(false);
  let logFile = $state('');
  let logs = $state<string[]>([]);
  let installDone = $state(false);
  let installFailed = $state(false);
  let installing = $state(false);
  let err = $state('');

  // 配置
  let storageBackend = $state('btrfs');
  let storageSize = $state(50);
  let storagePool = $state('default');
  let networkBridge = $state('incusbr0');
  let installMethod = $state<'auto' | 'package' | 'source'>('auto');

  let pollTimer: ReturnType<typeof setInterval> | null = null;
  let logScrollEl = $state<HTMLDivElement | null>(null);
  /** 连续读取日志失败/文件缺失的次数：超过阈值就停止轮询并报错，避免无限转圈 */
  let pollFailures = 0;

  const steps = ['系统检测', '配置', '安装中', '完成'];

  onMount(() => {
    // 若安装中刷新过页面，从 sessionStorage 恢复日志跟踪（后台脚本仍在执行）
    const saved = sessionStorage.getItem('incus-install-log');
    if (saved) {
      logFile = saved;
      step = 3;
      installing = true;
      pollLog();
      pollTimer = setInterval(pollLog, 2000);
    } else {
      doCheck();
    }
  });

  onDestroy(() => {
    if (pollTimer) clearInterval(pollTimer);
  });

  async function doCheck() {
    checking = true;
    err = '';
    try {
      // request() 已解包 ApiResponse.data，返回的就是状态对象本身
      const res = await api.installStatus();
      status = res;
      installState.checked = true;
      installState.ready = Boolean(res?.installed && res?.initialized);
      if (installState.ready) {
        step = 4;
      }
    } catch (e) {
      err = e instanceof Error ? e.message : String(e);
    } finally {
      checking = false;
    }
  }

  async function startInstall() {
    if (installing) return;
    step = 3;
    err = '';
    logs = [];
    installDone = false;
    installFailed = false;
    installing = true;
    try {
      const res = await api.installIncus({
        storage_backend: storageBackend,
        storage_size_gb: storageSize,
        storage_pool: storagePool,
        network_bridge: networkBridge,
        install_method: installMethod,
      });
      // 没有日志路径就无法跟踪进度，直接报错，否则界面会永远停在「正在安装」且不轮询
      if (!res?.log_file) throw new Error('后端未返回安装日志路径，无法跟踪安装进度');
      logFile = res.log_file;
      sessionStorage.setItem('incus-install-log', logFile);
      await pollLog();
      pollTimer = setInterval(pollLog, 2000);
    } catch (e) {
      err = e instanceof Error ? e.message : String(e);
      installing = false;
      step = 2;
    }
  }

  async function pollLog() {
    if (!logFile) return;
    try {
      const res = await api.installLog(logFile);
      if (res.exists) {
        pollFailures = 0;
        logs = res.lines;
        requestAnimationFrame(() => {
          if (logScrollEl) logScrollEl.scrollTop = logScrollEl.scrollHeight;
        });
        if (res.done && pollTimer) {
          clearInterval(pollTimer);
          pollTimer = null;
          installing = false;
          sessionStorage.removeItem('incus-install-log');
          if (res.success) {
            installDone = true;
            await doCheck();
            // doCheck 只在 installed && initialized 时才跳到第 4 步。
            // 脚本 rc=0 但复检仍未就绪（例如存储池没建成）时留在本页并标记失败：
            // 日志仍可见、有重试按钮，否则界面会永远停在「正在安装」且无处可点。
            if (step === 3) {
              installDone = false;
              installFailed = true;
              err = '安装脚本报告成功，但复检显示 Incus 仍未初始化。请查看日志中的 [WARN]/[ERROR] 行，或返回配置页调整参数后重试。';
            }
          } else {
            installFailed = true;
            err = '安装脚本执行失败，请查看日志中的 [ERROR] 行定位原因';
          }
        }
        return;
      }
      // 日志文件暂时不存在：可能脚本刚启动，给它一点时间
      if (++pollFailures >= 10) failPolling('安装日志文件不存在，无法跟踪安装进度');
    } catch (e) {
      if (++pollFailures >= 10) failPolling(`无法读取安装日志：${e instanceof Error ? e.message : String(e)}`);
    }
  }

  /** 停止轮询并展示错误（原实现 catch 里静默忽略，异常时会永远停在「正在安装」） */
  function failPolling(message: string) {
    if (pollTimer) {
      clearInterval(pollTimer);
      pollTimer = null;
    }
    installing = false;
    installFailed = true;
    err = message;
    // 不清掉的话刷新页面又会恢复到第 3 步继续失败，用户被永久困在「正在安装」
    sessionStorage.removeItem('incus-install-log');
  }

  function backToConfig() {
    installFailed = false;
    err = '';
    step = 2;
  }

  function goDashboard() {
    navigate('/dashboard', true);
  }

  // 解析日志行的级别颜色
  function logClass(line: string): string {
    if (line.includes('[ERROR]')) return 'text-red-400';
    if (line.includes('[WARN]')) return 'text-yellow-400';
    if (line.includes('[OK]')) return 'text-green-400';
    if (line.includes('[DONE]')) return 'text-blue-400 font-bold';
    if (line.includes('[STEP]')) return 'text-cyan-400';
    return 'text-gray-300';
  }
</script>

<div class="max-w-3xl mx-auto px-4 py-8">
  <h1 class="text-2xl font-bold mb-6 text-gray-900 dark:text-white">Incus 安装向导</h1>

  <!-- 步骤条 -->
  <div class="flex items-center mb-8">
    {#each steps as label, i}
      <div class="flex-1 flex items-center">
        <div class="flex flex-col items-center">
          <div class="w-8 h-8 rounded-full flex items-center justify-center text-sm font-bold transition-colors
            {step > i + 1 ? 'bg-green-500 text-white' : step === i + 1 ? 'bg-blue-600 text-white' : 'bg-gray-200 text-gray-500 dark:bg-gray-700 dark:text-gray-400'}">
            {i + 1}
          </div>
          <span class="text-xs mt-1 {step === i + 1 ? 'text-blue-600 font-medium dark:text-blue-400' : 'text-gray-500 dark:text-gray-400'}">{label}</span>
        </div>
        {#if i < steps.length - 1}
          <div class="flex-1 h-1 mx-2 rounded {step > i + 1 ? 'bg-green-500' : 'bg-gray-200 dark:bg-gray-700'}"></div>
        {/if}
      </div>
    {/each}
  </div>

  {#if err}
    <div class="bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 text-red-700 dark:text-red-300 px-4 py-3 rounded mb-4 text-sm">
      {err}
    </div>
  {/if}

  <!-- ====== Step 1: 系统检测 ====== -->
  {#if step === 1}
    <div class="bg-white dark:bg-gray-800 rounded-lg shadow p-6">
      <h2 class="text-lg font-semibold mb-4 text-gray-900 dark:text-white">系统检测</h2>
      {#if checking}
        <div class="flex items-center justify-center py-10">
          <div class="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-600"></div>
          <span class="ml-3 text-gray-600 dark:text-gray-300">检测宿主机环境...</span>
        </div>
      {:else if status}
        <div class="space-y-3">
          <div class="flex justify-between border-b border-gray-100 dark:border-gray-700 pb-2">
            <span class="text-gray-500 dark:text-gray-400">操作系统</span>
            <span class="font-medium text-gray-900 dark:text-white">{status.os_name || '未知'} {status.os_version || ''}</span>
          </div>
          <div class="flex justify-between border-b border-gray-100 dark:border-gray-700 pb-2">
            <span class="text-gray-500 dark:text-gray-400">Incus 安装状态</span>
            <span class="font-medium {status.installed ? 'text-green-600 dark:text-green-400' : 'text-red-600 dark:text-red-400'}">
              {status.installed ? '已安装' : '未安装'}
            </span>
          </div>
          {#if status.installed}
            <div class="flex justify-between border-b border-gray-100 dark:border-gray-700 pb-2">
              <span class="text-gray-500 dark:text-gray-400">版本</span>
              <span class="font-medium text-gray-900 dark:text-white">{status.version || '-'}</span>
            </div>
            <div class="flex justify-between border-b border-gray-100 dark:border-gray-700 pb-2">
              <span class="text-gray-500 dark:text-gray-400">服务状态</span>
              <span class="font-medium {status.service_running ? 'text-green-600 dark:text-green-400' : 'text-red-600 dark:text-red-400'}">
                {status.service_running ? '运行中' : '未运行'}
              </span>
            </div>
            <div class="flex justify-between border-b border-gray-100 dark:border-gray-700 pb-2">
              <span class="text-gray-500 dark:text-gray-400">初始化状态</span>
              <span class="font-medium {status.initialized ? 'text-green-600 dark:text-green-400' : 'text-red-600 dark:text-red-400'}">
                {status.initialized ? '已初始化' : '未初始化'}
              </span>
            </div>
          {/if}
          {#if status.socket_path}
            <div class="flex justify-between border-b border-gray-100 dark:border-gray-700 pb-2">
              <span class="text-gray-500 dark:text-gray-400">Incus Socket</span>
              <span class="font-mono text-xs text-gray-700 dark:text-gray-200 break-all text-right">{status.socket_path}</span>
            </div>
          {/if}
        </div>

        {#if status.installed && status.api_reachable === false}
          <div class="mt-4 bg-amber-50 dark:bg-amber-900/20 border border-amber-200 dark:border-amber-800 text-amber-800 dark:text-amber-200 px-4 py-3 rounded text-sm">
            Incus 已安装，但面板连不上它的 Unix socket（{status.socket_path}）。除本页外，面板其他功能都会返回 500。
            请确认服务已启动（<code class="font-mono">systemctl status incus</code>）；
            若 socket 在别处，可用环境变量 <code class="font-mono">INCUS_SOCKET</code> 指定后重启面板。
          </div>
        {/if}

        {#if status.is_root === false}
          <div class="mt-4 bg-amber-50 dark:bg-amber-900/20 border border-amber-200 dark:border-amber-800 text-amber-800 dark:text-amber-200 px-4 py-3 rounded text-sm">
            面板当前不是以 root 身份运行，无法执行安装脚本（需要调用 apt/dnf、systemctl 并写入 /etc）。
            请用 root 重新启动面板，或按 <code class="font-mono">linuxcontainers.org/incus</code> 的说明手动安装 Incus 后回到本页复检。
          </div>
        {/if}

        <div class="mt-6 flex justify-end gap-3">
          <button class="px-4 py-2 text-sm border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-300 rounded-md hover:bg-gray-50 dark:hover:bg-gray-700" onclick={doCheck}>
            重新检测
          </button>
          {#if status.installed && status.initialized}
            <button class="px-4 py-2 text-sm bg-green-600 hover:bg-green-700 text-white rounded-md" onclick={goDashboard}>
              进入面板
            </button>
          {:else}
            <button
              class="px-4 py-2 text-sm bg-blue-600 hover:bg-blue-700 text-white rounded-md disabled:cursor-not-allowed disabled:opacity-50"
              disabled={status.is_root === false}
              onclick={() => step = 2}
            >
              下一步：配置安装参数
            </button>
          {/if}
        </div>
      {/if}
    </div>
  {/if}

  <!-- ====== Step 2: 配置 ====== -->
  {#if step === 2}
    <div class="bg-white dark:bg-gray-800 rounded-lg shadow p-6">
      <h2 class="text-lg font-semibold mb-4 text-gray-900 dark:text-white">安装配置</h2>
      <div class="space-y-5">
        <div>
          <label for="iw-method" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">安装方式</label>
          <select id="iw-method" bind:value={installMethod} class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-white text-sm focus:ring-2 focus:ring-blue-500 focus:border-blue-500 outline-none">
            <option value="auto">自动（优先包管理器，失败回退源码编译）</option>
            <option value="package">仅包管理器（发行版自带/第三方源）</option>
            <option value="source">源码编译（通用，但耗时较长）</option>
          </select>
          <p class="text-xs text-gray-500 dark:text-gray-400 mt-1">
            {#if installMethod === 'source'}
              将从 GitHub 拉取最新稳定版源码编译，适合 Debian/Ubuntu 以外的发行版（Arch/Fedora/CentOS/Alpine 等）。
            {:else if installMethod === 'package'}
              Debian/Ubuntu 走 zabbly 源；Arch 用 pacman；Alpine 用 apk；Fedora/Rocky/CentOS 走 COPR；openSUSE 用 zypper。
            {:else}
              脚本会先尝试用发行版包管理器安装，若源中没有则自动改为源码编译。
            {/if}
          </p>
        </div>

        <div>
          <label for="iw-backend" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">存储后端</label>
          <select id="iw-backend" bind:value={storageBackend} class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-white text-sm focus:ring-2 focus:ring-blue-500 focus:border-blue-500 outline-none">
            <option value="btrfs">btrfs（推荐，CoW 秒级快照）</option>
            <option value="zfs">zfs（高性能，需内核模块）</option>
            <option value="dir">dir（兼容最好，无额外依赖）</option>
          </select>
          <p class="text-xs text-gray-500 dark:text-gray-400 mt-1">
            {#if storageBackend === 'btrfs'}
              写时复制文件系统，快照耗时与数据量无关，适合频繁备份场景。
            {:else if storageBackend === 'zfs'}
              企业级文件系统，支持压缩和去重，但需要内核模块支持。
            {:else}
              简单目录复制，快照会全量拷贝数据，速度随数据量增长。
            {/if}
          </p>
        </div>

        <div>
          <label for="iw-size" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">存储池大小 (GB)</label>
          <input id="iw-size" type="number" min="1" max="10000" bind:value={storageSize}
            class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-white text-sm focus:ring-2 focus:ring-blue-500 outline-none" />
          <p class="text-xs text-gray-500 dark:text-gray-400 mt-1">在无空闲磁盘时，会自动创建稀疏文件作为存储池。</p>
        </div>

        <div>
          <label for="iw-pool" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">存储池名称</label>
          <input id="iw-pool" type="text" bind:value={storagePool}
            class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-white text-sm focus:ring-2 focus:ring-blue-500 outline-none" />
        </div>

        <div>
          <label for="iw-bridge" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">网桥名称</label>
          <input id="iw-bridge" type="text" bind:value={networkBridge}
            class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-white text-sm focus:ring-2 focus:ring-blue-500 outline-none" />
          <p class="text-xs text-gray-500 dark:text-gray-400 mt-1">容器默认使用的 NAT 网桥，安装后会自动配置 DHCP 和地址转发。</p>
        </div>
      </div>

      <div class="mt-6 flex justify-between">
        <button class="px-4 py-2 text-sm border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-300 rounded-md hover:bg-gray-50 dark:hover:bg-gray-700" onclick={() => step = 1}>
          上一步
        </button>
        <button class="px-4 py-2 text-sm bg-blue-600 hover:bg-blue-700 text-white rounded-md" onclick={startInstall}>
          开始安装
        </button>
      </div>
    </div>
  {/if}

  <!-- ====== Step 3: 安装中 ====== -->
  {#if step === 3}
    <div class="bg-white dark:bg-gray-800 rounded-lg shadow p-6">
      <h2 class="text-lg font-semibold mb-4 text-gray-900 dark:text-white">
        {#if installFailed}安装未完成{:else if installDone}安装完成{:else}正在安装 Incus...{/if}
      </h2>
      <div class="w-full bg-gray-200 dark:bg-gray-700 rounded-full h-2 mb-4">
        <div class="{installFailed ? 'bg-red-500' : 'bg-blue-600 animate-pulse'} h-2 rounded-full" style="width: 100%"></div>
      </div>
      <p class="text-sm text-gray-600 dark:text-gray-300 mb-3">
        {#if installFailed}
          <span class="font-medium text-red-600 dark:text-red-400">安装失败，请根据下方日志排查问题。</span>
        {:else if installDone}
          安装完成，正在刷新状态...
        {:else}
          正在执行安装脚本，请勿关闭页面...
        {/if}
      </p>
      {#if installFailed}
        <button class="mb-3 px-4 py-2 text-sm border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-300 rounded-md hover:bg-gray-50 dark:hover:bg-gray-700" onclick={backToConfig}>
          返回配置页调整参数后重试
        </button>
      {/if}
      <div bind:this={logScrollEl} class="bg-gray-900 text-gray-300 font-mono text-xs p-4 rounded-md h-96 overflow-y-auto border border-gray-700">
        {#if logs.length === 0}
          <span class="text-gray-500">等待日志输出...</span>
        {:else}
          {#each logs as line}
            <div class="whitespace-pre-wrap break-all {logClass(line)}">{line}</div>
          {/each}
        {/if}
      </div>
    </div>
  {/if}

  <!-- ====== Step 4: 完成 ====== -->
  {#if step === 4}
    <div class="bg-white dark:bg-gray-800 rounded-lg shadow p-6 text-center">
      <div class="w-16 h-16 bg-green-100 dark:bg-green-900/30 rounded-full flex items-center justify-center mx-auto mb-4">
        <svg class="w-8 h-8 text-green-600 dark:text-green-400" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7"></path></svg>
      </div>
      <h2 class="text-xl font-bold text-gray-900 dark:text-white mb-2">安装完成</h2>
      {#if status}
        <p class="text-gray-600 dark:text-gray-300 mb-1">Incus {status.version || ''} 已就绪</p>
        <p class="text-gray-500 dark:text-gray-400 text-sm mb-6">
          存储池: <span class="font-medium text-gray-700 dark:text-gray-200">{storagePool}</span>
          &nbsp;|&nbsp;
          网桥: <span class="font-medium text-gray-700 dark:text-gray-200">{networkBridge}</span>
        </p>
      {/if}
      <button class="px-6 py-2 bg-blue-600 hover:bg-blue-700 text-white rounded-md text-base" onclick={goDashboard}>
        进入面板
      </button>
    </div>
  {/if}
</div>
