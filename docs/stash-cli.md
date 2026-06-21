# Stash CLI 使用说明

`stash-cli` 是本地终端版 Stash 入口。它直接打开已有的 Stash SQLite 数据库，不启动 Stash server，也不会创建新的 SQLite 数据库。写入数据库前请先停止正在使用同一数据库的 Stash server。

## 构建与启动

```bash
make stash-cli
mkdir -p ~/.config/stash-cli
go run ./cmd/stash-cli --print-config > ~/.config/stash-cli/config.toml
go run ./cmd/stash-cli --check
go run ./cmd/stash-cli
```

`stash-cli` 默认读取 `~/.config/stash-cli/config.toml`（按系统 `UserConfigDir` 解析）。只有需要临时使用其他配置文件时，才传 `-c/--config <path>`。

`--check` 只验证配置和数据库，并在启用 `scan_on_startup` 时执行启动扫描；不进入 TUI。

## 配置示例

```toml
database_path = "/home/tan/.stash/stash-go.sqlite"
media_dirs = ["/mnt/media/videos", "/run/media/tan/remote/videos"]
scan_on_startup = true
graphics_mode = "auto"
cache_dir = "~/.cache/stash-cli"
log_file = "~/.local/state/stash-cli/stash-cli.log"
log_level = "info"
log_stdout = false
ffprobe_path = "/usr/bin/ffprobe"
ffplay_path = "/usr/bin/ffplay"
ffplay_args = ["-autoexit", "-hide_banner", "-loglevel", "warning"]

[blobs]
# 必须和 Stash server 的 blob 配置一致。server 使用 FILESYSTEM 时填 filesystem。
storage = "filesystem"
path = "/home/tan/.stash/blobs"
```

`graphics_mode` 支持 `auto` 和 `kitty`。当前 TUI 使用封面网格；`kitty` 会强制使用 Kitty graphics，适合 Kitty/Ghostty 等已确认支持 Kitty 协议的终端；`auto` 会依赖终端探测，失败时可能回退到 glyph。远程视频请先通过 `sshfs`、NFS、SMB 等方式挂载成本机路径，再写入 `media_dirs`。

`log_stdout = false` 会把日志只写入 `log_file`，避免破坏 TUI 画面。调试时可临时设为 `true` 或用 `tail -f ~/.local/state/stash-cli/stash-cli.log` 观察日志。

`database_path` 必须指向 Stash server 已经初始化过的 sqlite 文件。`stash-cli` 不负责创建或迁移数据库；如果文件不存在，请先运行 Stash server 完成初始化。

`[blobs]` 必须和同一个 Stash server 的 blob 配置匹配。主 Stash 的 `blobs_storage: FILESYSTEM` 对应 `storage = "filesystem"` 和相同的 `blobs_path`；如果主 Stash 使用数据库存储 blob，则使用 `storage = "database"`。

CLI 只读取 Stash 数据库中已有的封面，不通过 stash-box 抓取封面，也不通过 ffmpeg 生成封面。

## TUI 操作

TUI 使用 Vim 风格 normal 模式。`h/j/k/l` 或方向键移动当前网格，`enter` 使用 `ffplay_path` 播放当前选中的 scene，`space` 打开当前 scene 的详情视图。播放期间状态栏会显示轻量进度条；`ffplay` 正常退出后，stash-cli 会给该 scene 增加一次 view history，用作播放次数记录。按 `:` 进入底部命令输入栏，`esc` 取消输入，`:q` 退出。

scene 网格卡片显示封面、标题、至少一名 performer，以及短元信息，例如 `1.2h/2012`。日期只显示年份。

详情视图顶部以两列显示 scene 信息，下面用网格列出该 scene 的 performers。performer 卡片会读取 Stash 数据库中的 performer image blob 作为头像，并以两列短格式显示出生年份、身高、拍摄年份年龄、从业年份和 scene 数量，例如 `birth: 86`、`career: 08-`。没有头像时不显示占位文字。详情视图支持：

- `h/j/k/l`：在 performer 网格中移动焦点。
- `enter`：跳转到当前 performer 的全部 scenes。
- `:rating <score>`：给当前选中的 performer 设置 rating，范围 `0..100`。
- `esc`：关闭详情面板并返回 scene 网格。

命令输入栏支持：

- `search <query>`：搜索名称，也支持 `tag:<name>`、`performer:<name>`、`actress:<name>`。
- `random <n>`：随机列出 `n` 个 scene；不传 `n` 时，按当前终端窗口可显示的网格数量决定。
- `default`：清除搜索条件并回到默认 scene 网格。
- `rating <score>`：给当前选中的 scene 或 performer 设置 rating，范围 `0..100`。
- `delete`：删除当前选中的 scene 或 performer；需要再按 `y` 确认。
- `clear`：清除搜索条件。
- `scan`：按配置的 `media_dirs` 扫描本地或已挂载的远程目录。
- `kitty`：在当前会话内切换 graphics mode；再次执行会在 `auto` 和强制 `kitty` 之间切换。
- `help`：显示命令列表。
- `:q`：退出。

第一版不会执行 tag/studio 管理、scraper、文件移动或终端内视频播放。
