import io

p = 'Images.svelte'
s = io.open(p, encoding='utf-8').read()

# 1) 远程 form 打开处加模式切换
old = '''    <form id="import-form" class="grid gap-4" onsubmit={submitImport}>'''
new = '''    <div class="mb-4 flex items-center gap-1 rounded-lg bg-muted p-1">
      <button
        type="button"
        class="h-7 flex-1 rounded-md px-3 text-[12.5px] font-medium transition-colors {importMode === 'remote'
          ? 'bg-background text-foreground shadow-sm'
          : 'text-muted-foreground hover:text-foreground'}"
        onclick={() => (importMode = 'remote')}
      >
        从远程源拉取
      </button>
      <button
        type="button"
        class="h-7 flex-1 rounded-md px-3 text-[12.5px] font-medium transition-colors {importMode === 'local'
          ? 'bg-background text-foreground shadow-sm'
          : 'text-muted-foreground hover:text-foreground'}"
        onclick={() => (importMode = 'local')}
      >
        上传本地镜像文件
      </button>
    </div>

    {#if importMode === 'remote'}
    <form id="import-form" class="grid gap-4" onsubmit={submitImport}>'''
assert old in s, 'form open'
s = s.replace(old, new)

# 2) 远程 form 结束 → 本地上传分支。导入 form 的结束是 </form> 后跟 </Dialog.Content>
old = '''    </form>
  </Dialog.Content>
</Dialog.Root>

<!-- 远程源管理 -->'''
new = '''    </form>
    {:else}
    <div class="space-y-4">
      <input
        type="file"
        bind:this={uploadInputEl}
        onchange={onPickUploadFile}
        accept=".tar.gz,.tgz,.tar.xz,.tar.bz2,.tar,.qcow2,.img"
        class="hidden"
      />
      <button
        type="button"
        class="flex w-full flex-col items-center justify-center gap-2 rounded-xl border border-dashed border-border/80 px-4 py-8 transition-colors hover:border-primary/40 hover:bg-foreground/[0.02]"
        onclick={() => uploadInputEl?.click()}
      >
        <Upload size={22} class="text-primary" />
        <span class="text-[13px] font-medium">
          {uploadFile ? uploadFile.name : '点击选择镜像文件'}
        </span>
        <span class="text-[11px] text-muted-foreground">
          {#if uploadFile}
            {fmtSize(uploadFile.size)} · 支持合并格式 tarball（tar.gz / tar.xz）与 qcow2
          {:else}
            支持合并格式镜像包：tar.gz / tar.xz / tar.bz2 / qcow2
          {/if}
        </span>
      </button>

      {#if uploadingLocal}
        <div>
          <div class="mb-1 flex justify-between text-[11.5px] text-muted-foreground">
            <span>上传中…</span>
            <span class="font-num">{uploadProgress}%</span>
          </div>
          <div class="h-[6px] overflow-hidden rounded-full bg-foreground/[0.05]">
            <div class="h-full rounded-full bg-primary transition-[width] duration-200" style="width:{uploadProgress}%"></div>
          </div>
        </div>
      {/if}

      <Button class="h-9 w-full gap-1.5" disabled={!uploadFile || uploadingLocal} onclick={doUploadLocal}>
        {#if uploadingLocal}
          <span class="mr-1 inline-block size-3.5 animate-spin rounded-full border-2 border-current border-t-transparent"></span>
          上传中（{uploadProgress}%）
        {:else}
          <Upload size={14} /> 开始上传并导入
        {/if}
      </Button>
      <p class="text-[11px] leading-relaxed text-muted-foreground">
        上传到面板所在机器后由 Incus 解包导入，导入完成后镜像会以指纹标识出现在列表中。
      </p>
    </div>
    {/if}
  </Dialog.Content>
</Dialog.Root>

<!-- 远程源管理 -->'''
assert old in s, 'form close'
s = s.replace(old, new)

io.open(p, 'w', encoding='utf-8', newline='\n').write(s)
print('form split done')
