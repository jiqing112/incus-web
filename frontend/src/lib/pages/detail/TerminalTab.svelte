<script lang="ts">
  import { onMount } from 'svelte';
  import '@xterm/xterm/css/xterm.css';
  import { Terminal } from '@xterm/xterm';
  import { FitAddon } from '@xterm/addon-fit';
  import { Card } from '$lib/components/ui/card';
  import { Button } from '$lib/components/ui/button';
  import { RotateCw, Expand, Shrink } from '@lucide/svelte';
  import { api } from '$lib/api';
  import { toast } from 'svelte-sonner';

  let { name }: { name: string } = $props();

  // 与 api.ts 同源：vite.config.ts 启动时注入，生产构建恒为 false
  const MOCK = __MOCK__;

  let holder: HTMLDivElement | null = $state(null);
  let statusEl = $state<'connecting' | 'connected' | 'closed' | 'error'>('connecting');

  let term: Terminal | null = null;
  let fitAddon: FitAddon | null = null;
  let ws: WebSocket | null = null;
  let resizeObs: ResizeObserver | null = null;

  const xtermTheme = {
    background: '#0b1220',
    foreground: '#d5dde8',
    cursor: '#4fc3f7',
    selectionBackground: '#264f78',
    black: '#1c2733',
    red: '#f7768e',
    green: '#69f0ae',
    yellow: '#ffd740',
    blue: '#7aa2f7',
    magenta: '#bb9af7',
    cyan: '#7dcfff',
    white: '#d5dde8',
  };

  function sendResize() {
    if (!term || !ws || ws.readyState !== WebSocket.OPEN) return;
    ws.send(JSON.stringify({ type: 'resize', width: term.cols, height: term.rows }));
  }

  onMount(() => {
    term = new Terminal({
      fontFamily: "'JetBrains Mono Variable', ui-monospace, Consolas, monospace",
      fontSize: 13.5,
      lineHeight: 1.25,
      cursorBlink: true,
      allowProposedApi: true,
      scrollback: 4000,
      theme: xtermTheme,
    });
    fitAddon = new FitAddon();
    term.loadAddon(fitAddon);
    if (holder) {
      term.open(holder);
      fitAddon.fit();
    }

    // 输入转发只注册一次：重连时重复注册会让每个字符被发送多次
    term.onData((d) => {
      if (MOCK) {
        if (mockActive) handleMockData(d);
        return;
      }
      if (ws?.readyState === WebSocket.OPEN) ws.send(d);
    });

    if (MOCK) {
      statusEl = 'connected';
      runMockShell();
    } else {
      connectWS();
    }

    resizeObs = new ResizeObserver(() => {
      try {
        fitAddon?.fit();
        sendResize();
      } catch {
        /* 面板隐藏时 fit 可能抛错，忽略 */
      }
    });
    if (holder?.parentElement) resizeObs.observe(holder.parentElement);

    return () => {
      resizeObs?.disconnect();
      ws?.close();
      term?.dispose();
      ws = null;
      term = null;
    };
  });

  function connectWS() {
    statusEl = 'connecting';
    let sock: WebSocket;
    try {
      sock = new WebSocket(api.terminalURL(name));
    } catch (e) {
      statusEl = 'error';
      toast.error(e instanceof Error ? e.message : '无法建立终端连接');
      return;
    }
    ws = sock;
    sock.binaryType = 'arraybuffer';

    // 事件里只认当前 socket：重连后旧连接的 onclose 不能覆盖新会话状态
    sock.onopen = () => {
      if (ws !== sock) return;
      statusEl = 'connected';
      sendResize();
      term?.writeln('\x1b[90m—— 已连接到实例终端，输入命令开始 ——\x1b[0m\r\n');
      term?.focus();
    };
    sock.onmessage = (e) => {
      if (ws !== sock) return;
      if (typeof e.data === 'string') term?.write(e.data);
      else term?.write(new Uint8Array(e.data));
    };
    sock.onerror = () => {
      if (ws !== sock) return;
      statusEl = 'error';
      term?.writeln('\r\n\x1b[31m连接出错，实例可能未运行或 exec 不可用。\x1b[0m\r\n');
    };
    sock.onclose = () => {
      if (ws !== sock) return;
      ws = null;
      if (statusEl !== 'error') statusEl = 'closed';
      term?.writeln('\r\n\x1b[90m—— 会话已断开 ——\x1b[0m\r\n');
    };
  }

  function reconnect() {
    const old = ws;
    ws = null;
    old?.close();
    term?.reset();
    mockActive = false;
    if (MOCK) {
      statusEl = 'connected';
      runMockShell();
    } else {
      connectWS();
    }
  }

  /* ------- mock 模拟 shell（仅演示用） ------- */
  let mockActive = false;
  let mockLine = '';
  let mockCwd = '~';
  const MOCK_FILES: Record<string, string[]> = {
    '~': ['app/', '.bashrc', 'README.md', 'notes.txt'],
    '/': ['bin/', 'etc/', 'home/', 'root/', 'tmp/', 'var/'],
    '~/app': ['main.py', 'requirements.txt', '.env'],
  };

  function runMockShell() {
    const t = term;
    if (!t) return;
    mockActive = true;
    mockLine = '';
    mockCwd = '~';
    t.reset();
    const prompt = `\r\n\x1b[1;32mroot@${name}\x1b[0m:\x1b[1;34m~\x1b[0m# `;
    t.writeln('\x1b[90m（演示模式：这是模拟 shell，连接真实 Incus 后即为真实终端）\x1b[0m');
    t.write(prompt);
  }

  function handleMockData(d: string) {
    const t = term;
    if (!t) return;
    if (d === '\r' || d === '\n') {
      const cmd = mockLine.trim();
      mockLine = '';
      t.write('\r\n');
      if (cmd) {
        const [bin, ...args] = cmd.split(/\s+/);
        switch (bin) {
          case 'ls':
            t.write((MOCK_FILES[mockCwd] ?? []).join('  '));
            t.write('\r\n');
            break;
          case 'pwd':
            t.write(mockCwd === '~' ? '/root' : mockCwd);
            t.write('\r\n');
            break;
          case 'cd':
            mockCwd = args[0] ?? '~';
            break;
          case 'uname':
            t.write(args[0] === '-a' ? `Linux ${name} 6.8.0-45-generic #44-Ubuntu SMP x86_64 GNU/Linux` : 'Linux');
            t.write('\r\n');
            break;
          case 'echo':
            t.write(args.join(' '));
            t.write('\r\n');
            break;
          case 'clear':
            t.clear();
            break;
          case 'exit':
            t.writeln('logout');
            return;
          case 'help':
            t.writeln('演示 shell：支持 ls / cd / pwd / uname / echo / clear / exit');
            break;
          default:
            t.write(`sh: ${bin}: command not found`);
            t.write('\r\n');
        }
      }
      t.write(`\x1b[1;32mroot@${name}\x1b[0m:\x1b[1;34m${mockCwd}\x1b[0m# `);
    } else if (d === '\x7f') {
      if (mockLine.length > 0) {
        mockLine = mockLine.slice(0, -1);
        t.write('\b \b');
      }
    } else if (d >= ' ' || d === '\t') {
      mockLine += d;
      t.write(d === '\t' ? '  ' : d);
    }
  }

  let fullscreen = $state(false);
  function toggleFullscreen() {
    fullscreen = !fullscreen;
    setTimeout(() => {
      try {
        fitAddon?.fit();
      } catch { /* noop */ }
    }, 60);
  }
</script>

<Card class="overflow-hidden p-0 shadow-soft {fullscreen ? 'fixed inset-3 z-50 flex flex-col' : ''}">
  <div class="flex items-center gap-2 border-b border-border/60 bg-sidebar px-4 py-2.5">
    <span class="flex gap-1.5" aria-hidden="true">
      <span class="size-2.5 rounded-full bg-[#ff5f57]"></span>
      <span class="size-2.5 rounded-full bg-[#febc2e]"></span>
      <span class="size-2.5 rounded-full bg-[#28c840]"></span>
    </span>
    <span class="ml-1 font-mono text-[11.5px] text-muted-foreground">
      sh · {name}
    </span>
    <span class="ml-2 flex items-center gap-1.5 text-[11px]">
      {#if statusEl === 'connected'}
        <span class="relative flex h-[7px] w-[7px]">
          <span class="absolute inline-flex h-full w-full animate-ping rounded-full bg-success opacity-50"></span>
          <span class="relative inline-flex h-[7px] w-[7px] rounded-full bg-success"></span>
        </span>
        <span class="text-muted-foreground">已连接</span>
      {:else if statusEl === 'connecting'}
        <span class="size-[7px] rounded-full bg-warning"></span>
        <span class="text-muted-foreground">连接中…</span>
      {:else if statusEl === 'error'}
        <span class="size-[7px] rounded-full bg-destructive"></span>
        <span class="text-destructive">连接失败</span>
      {:else}
        <span class="size-[7px] rounded-full bg-muted-foreground/40"></span>
        <span class="text-muted-foreground">已断开</span>
      {/if}
    </span>

    <div class="ml-auto flex items-center gap-0.5">
      <Button variant="ghost" size="icon" class="size-7.5 text-muted-foreground" onclick={reconnect} aria-label="重新连接" title="重新连接">
        <RotateCw size={14} />
      </Button>
      <Button variant="ghost" size="icon" class="size-7.5 text-muted-foreground" onclick={toggleFullscreen} aria-label="全屏" title="全屏">
        {#if fullscreen}<Shrink size={14} />{:else}<Expand size={14} />{/if}
      </Button>
    </div>
  </div>

  <div class="bg-[#0b1220] p-2 {fullscreen ? 'min-h-0 flex-1' : 'h-[480px]'}">
    <div bind:this={holder} class="h-full w-full"></div>
  </div>
</Card>
