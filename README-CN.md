# Apple Music ALAC / 杜比全景声下载器

[English](./README.md) | [简体中文](./README-CN.md) | [云服务器 / 代理配置](./PROXY-SETUP.md)

> **原脚本由 Sorrow 编写。** 本仓库已修改并包含修复与改进。

这是一个命令行工具，可从 Apple Music 下载专辑、单曲、播放列表、电台和音乐视频，支持 ALAC、AAC 和杜比全景声，并能保留或嵌入元数据与歌词。请仅用于你有权访问的内容，并遵守 Apple 条款和适用法律。

## 目录

- [功能特性](#功能特性)
- [支持的格式](#支持的格式)
- [前置要求](#前置要求)
- [快速开始](#快速开始)
- [图形界面](#图形界面)
- [配置](#配置)
- [使用方法](#使用方法)
- [升级](#升级)
- [开发者指南](#开发者指南)
- [获取 media-user-token](#获取-media-user-token)
- [歌词设置](#歌词设置)
- [隐私与安全](#隐私与安全)
- [致谢](#致谢)
- [云服务器 / 代理配置](./PROXY-SETUP.md)

## 功能特性

1. 内嵌封面和 LRC 歌词。
2. 逐词歌词和未同步歌词。
3. 歌手全部专辑下载。
4. 大文件流式下载和解密。
5. 音乐视频下载，使用进程内 mp4ff 解密。
6. 交互式搜索和曲目选择。

## 支持的格式

| 格式 | 描述 | 需要订阅 |
|---|---|---|
| `alac` | `audio-alac-stereo` | 是 |
| `ec3` | `audio-atmos` / `audio-ec3` | 是 |
| `aac` | `audio-stereo` | 是 |
| `aac-lc` | `audio-stereo` | 是 |
| `aac-binaural` | `audio-stereo-binaural` | 是 |
| `aac-downmix` | `audio-stereo-downmix` | 是 |
| `MV` | 音乐视频 | 是 |

下载电台需要来自有效订阅的 `media-user-token`。

## 前置要求

运行前必须准备：

1. **Apple Music 账号**：ALAC / AAC / 杜比全景声均需有效订阅，且该账号能正常登录 Apple Music。
2. **wrapper-lite**：必需的后端解密服务，未启动时任何下载都会失败。见 [wrapper-lite 仓库](https://github.com/WorldObservationLog/wrapper/tree/lite)。
   - **获取与启动**：推荐用 Release 内置的 `wrapper-lite-qemu` 启动器（自包含 QEMU，无需 root、无需原生 chroot）；也可用 Docker，或从源码用 CMake + C++ 工具链构建。
   - **首次登录**：首次使用必须先完成一次登录并把 token 缓存到本地（可能要求 2FA 验证码），之后才能以服务模式启动。
   - 默认监听 `http://127.0.0.1:12340`，把该地址写入 `config.yaml` 的 `general.lite-server`。
   - 若服务运行在另一台设备或容器中，需让它监听 `0.0.0.0`，并把 `lite-server` 换成那台机器的可达地址（不能写 `127.0.0.1`）。
   - 服务 `/status` 返回的 region 必须覆盖你要下载的 storefront。
3. **ffmpeg**（可选）：仅在后下载转换、动态封面或依赖 ffmpeg 的功能中需要。见 [ffmpeg.org](https://ffmpeg.org/)。
4. **网络**：需能访问 `music.apple.com` 与 `amp-api.music.apple.com`。云服务器等受限网络常被 Apple 屏蔽，见 [云服务器 / 代理配置](./PROXY-SETUP.md)。
5. **磁盘空间**：无损与音乐视频文件较大，下载为流式处理，请预留足够空间；内存缓冲上限可在 `general.max-memory-limit` 调整。

> **提示**：直接使用 Release 预编译二进制文件**不需要安装 Go**。只有自行从源码编译时才需要 Go 1.23.1+（见 [开发者指南](#开发者指南)）。
> Android / Termux 另需 `termux-setup-storage` 授权存储；批量或后台下载前建议使用 `termux-wake-lock`。

## 快速开始

从 [最新 GitHub Releases](https://github.com/zhaarey/apple-music-downloader/releases/latest) 下载适合你系统和架构的预编译二进制文件：

| 系统平台 | 硬件架构 | 预编译二进制文件名 |
|---|---|---|
| **Windows** | x86_64 (64位) | `amdl_windows_amd64.exe` |
| **Windows** | ARM64 | `amdl_windows_arm64.exe` |
| **macOS** | Apple Silicon (M1/M2/M3/M4) | `amdl_darwin_arm64` |
| **macOS** | Intel (x86_64) | `amdl_darwin_amd64` |
| **Linux** | x86_64 (amd64) | `amdl_linux_amd64` |
| **Linux** | ARM64 (aarch64) | `amdl_linux_arm64` |
| **Android (Termux)** | ARM64 (aarch64) | `amdl_android_arm64` |

---

### Windows (PowerShell)

1. 打开 PowerShell 终端，执行以下命令下载程序：

```powershell
# 下载预编译二进制（以 64 位 Windows 为例）
Invoke-WebRequest -Uri "https://github.com/zhaarey/apple-music-downloader/releases/latest/download/amdl_windows_amd64.exe" -OutFile "amdl.exe"
```

2. 首次运行 `amdl.exe`，程序将自动在当前目录下生成默认的 `config.yaml`：

```powershell
.\amdl.exe
```

3. 用文本编辑器打开生成的 `config.yaml`，按需配置 `lite-server` 地址与保存路径。
4. 运行测试：

```powershell
.\amdl.exe --help
```

下载示例：

```powershell
.\amdl.exe "https://music.apple.com/us/album/whenever-you-need-somebody-2022-remaster/1624945511"
```

---

### macOS

1. 下载预编译二进制：

```bash
# Apple Silicon 芯片（M 系列）：
curl -L -o amdl https://github.com/zhaarey/apple-music-downloader/releases/latest/download/amdl_darwin_arm64

# Intel 芯片：
# curl -L -o amdl https://github.com/zhaarey/apple-music-downloader/releases/latest/download/amdl_darwin_amd64

# 赋予执行权限
chmod +x amdl
```

> **macOS Gatekeeper 提示**：如果系统提示“无法打开，因为无法验证开发者”，请在终端执行以下命令移除隔离属性：
> ```bash
> xattr -d com.apple.quarantine amdl
> ```

2. 首次运行 `amdl`，程序将自动生成默认的 `config.yaml`：

```bash
./amdl
```

3. 编辑 `config.yaml` 填写你的配置。
4. 运行测试：

```bash
./amdl --help
```

下载示例：

```bash
./amdl "https://music.apple.com/us/album/whenever-you-need-somebody-2022-remaster/1624945511"
```

---

### Linux

1. 下载预编译二进制：

```bash
# x86_64 / amd64 架构：
curl -L -o amdl https://github.com/zhaarey/apple-music-downloader/releases/latest/download/amdl_linux_amd64

# ARM64 架构：
# curl -L -o amdl https://github.com/zhaarey/apple-music-downloader/releases/latest/download/amdl_linux_arm64

# 赋予执行权限
chmod +x amdl
```

2. 首次运行 `amdl`，程序将自动生成默认的 `config.yaml`：

```bash
./amdl
```

3. 编辑 `config.yaml` 填写你的配置。
4. 运行测试：

```bash
./amdl --help
```

下载示例：

```bash
./amdl "https://music.apple.com/us/album/whenever-you-need-somebody-2022-remaster/1624945511"
```

---

### Android / Termux

请从 [F-Droid](https://f-droid.org/en/packages/com.termux/) 或 [官方 GitHub Releases](https://github.com/termux/termux-app/releases) 安装 Termux（切勿使用 Google Play 商店的旧版）。

1. 更新软件包并安装 curl（ffmpeg 可选，用于格式转换或动态封面）：

```bash
pkg update && pkg upgrade
pkg install -y curl ffmpeg
```

2. 授权访问 Android 存储空间（同意权限后将创建 `~/storage/shared`）：

```bash
termux-setup-storage
```

3. 下载预编译二进制：

```bash
curl -L -o amdl https://github.com/zhaarey/apple-music-downloader/releases/latest/download/amdl_android_arm64
chmod +x amdl
```

4. 首次运行 `amdl`，程序将自动生成默认的 `config.yaml`：

```bash
./amdl
```

5. 编辑 `config.yaml`。若希望将音乐直接下载到系统音乐目录，可将配置中的路径设置为共享存储路径：

```yaml
paths:
  alac: "/sdcard/Music/amdl"
  atmos: "/sdcard/Music/amdl-atmos"
  aac: "/sdcard/Music/amdl-aac"
  mv: "/sdcard/Music/amdl-mv"
```

6. 正常下载：

```bash
./amdl "https://music.apple.com/us/album/whenever-you-need-somebody-2022-remaster/1624945511"
```

> **Termux 使用提示**：
> - 长时间或后台批量下载前，建议使用 `termux-wake-lock` 防止系统休眠，下载完成后执行 `termux-wake-unlock` 释放。
> - 如果 wrapper-lite 运行在局域网内其他设备上，`lite-server` 需填写该设备的局域网 IP，不能填写 `127.0.0.1`。
> - 本说明面向 Android arm64 环境；32 位 Android 不在文档支持范围内。
> - 旧版本自更新若报错 `lookup api.github.com on [::1]:53 ... connection refused`，请按第 3 步重新下载一次，此后即可正常自更新。

## 图形界面

**双击 `amdl.exe` 就会打开图形界面**，不需要加任何参数。

会弹出一个独立窗口（基于 Edge WebView2），流程是：

1. **粘贴链接** —— 支持专辑、单曲、播放列表、电台、音乐视频、歌手页面。
   带 `?i=` 的专辑链接只下载该单曲。
2. **自动获取信息** —— 显示封面、名称、艺人、年份、曲目数、厂牌、Apple Digital Master 标记，
   以及完整的曲目列表（含时长与脏标）。
3. **选择音质** —— 只列出**该内容实际支持**的音质：
   - 无损 (ALAC)：显示它是 CD 音质还是高解析无损
   - 高品质 (AAC)：256 kbps
   - 杜比全景声：内容不支持时置灰并说明原因
   - 下方实时显示该音质对应的保存目录
4. **确认下载** —— 显示进度条、已处理数量与实时日志。
5. **下载完成** —— 显示“已下载完成”、成功数量与文件列表，
   点击 **打开文件位置** 会在资源管理器中定位到文件。

### 双击与命令行的分工

程序会自己判断该开界面还是走命令行，**不需要记参数**：

| 启动方式 | 行为 |
|---|---|
| **双击 EXE**（或在开始菜单/快捷方式里点） | 打开图形界面 |
| 在终端运行 `amdl.exe`（无参数） | 打印用法说明（命令行模式不变） |
| `amdl.exe --gui` / `-g` | 强制打开图形界面 |
| `amdl.exe --gui=false` | 强制走命令行 |
| `amdl.exe <链接>` | 命令行下载（界面不会被打开） |
| `amdl.exe --search album 关键词` | 命令行搜索 |
| `amdl.exe --help` / `-h` | 打印用法说明 |
| 打错的参数，如 `amdl.exe --guy` | **打印用法说明**（不会静默开一个窗口把错误藏起来） |

> 判断依据是**当前进程是否与控制台共享终端**：双击时程序独占一个新控制台（无终端可读输出），
> 于是转为图形界面；从终端运行时共享终端的控制台，因此保持命令行行为。
>
> **双击启动失败时会弹窗提示**（双击场景没有终端可以打印错误）。
> 最常见的首次失败是缺少 `config.yaml`，弹窗会直接告诉你应该放在哪个目录。

### 启动与监听方式

| 项目 | 说明 |
|---|---|
| 启动方式 | **双击 `amdl.exe`**；或 `amdl.exe --gui`（简写 `-g`） |
| 监听地址 | `http://127.0.0.1:8722`，**仅回环地址**，局域网内其他设备无法访问 |
| 端口选择 | **固定端口 8722**（被占用时依次尝试 8723 / 8724，最后才用系统临时端口） |
| 寻址方法 | 终端会打印完整地址；因为端口固定，**书签和已打开的标签页重启后依然可用** |
| 窗口方式 | 优先用 Edge WebView2 开独立应用窗口；找不到时回退到 Edge / Chrome，最后回退到默认浏览器 |
| 停止服务 | **关闭该应用窗口**（进程随之退出）；也可在终端按 `Ctrl+C` |
| 断线处理 | 界面顶部会显示「与本地服务断开连接」并**自动重连**，恢复后自动刷新数据 |
| 配置文件位置 | 依次查找：**EXE 所在目录** → 当前工作目录。双击时工作目录不确定，所以以 EXE 目录优先 |
| 自定义配置 | `amdl.exe --gui -c D:\path\config.yaml` |

> **以前会出现 `Failed to fetch`，现已修复。** 早期版本每次启动都用**随机端口**，
> 所以程序一重启，之前打开的标签页就不再有效。现在地址固定，重启后刷新即可继续使用；
> 即使服务暂时不可用，界面也会给出明确提示并自行重连，不会只丢一个 `Failed to fetch`。
>
> 界面访问令牌保存在 `%APPDATA%\amdl\gui.json`（权限 0600，仅本机可读）。
> 删除该文件会在下次启动时重新生成端口与令牌。

### 下载历史持久化

下载记录保存在 `%APPDATA%\amdl\history.json`，**重启程序后依然可见**。

| 项目 | 说明 |
|---|---|
| 保存内容 | 名称、艺人、音质、**实际编码**（如 `ALAC 24-bit / 96000 Hz`）、开始与完成时间、成功/失败数量、保存目录 |
| 未保存内容 | 引擎实时日志与逐首文件列表（体积原因）——所以重启后记录里只有**打开位置**，没有文件清单 |
| 条数上限 | 最多 `300` 条，超出时自动丢弃最旧的记录 |
| 写入时机 | 每个任务**结束时写一次**，不是轮询时写，因此不会频繁读写磁盘 |
| 写入方式 | 先写 `.tmp` 再改名覆盖，中途断电不会把原文件写坏；文件损坏时直接重建为空 |
| 清空/删除 | 「清空历史」与单条「删除」会**同时**改内存与磁盘，重启后不会“复活” |
| 崩溃残留 | 状态为「进行中/排队中」的残留记录会在下次启动时自动剔除 |

> 历史文件只含本机下载元数据，**不含账号、密码或令牌**，可安全删除以重置历史。

### 鉴权

界面服务使用**持久化的 48 位令牌**鉴权（请求头 `X-AmDL-Token`）：

- 令牌只注入到本地页面中，其他程序或网页无法读取或驱动该接口。
- 未携带令牌的接口请求会被拒绝（HTTP 403）。
- 因此即使有其他程序扫描到该端口，也无法通过它发起下载或打开任意路径。

> 下载仍然需要 wrapper-lite 已启动并已登录（见 [前置要求](#前置要求)）。

## 配置

首次运行 `amdl` 时，如果当前目录下不存在配置文件，程序将自动从内置默认模板生成 `config.yaml`。

至少检查并设置：

```yaml
general:
  # wrapper-lite HTTP API 地址。
  lite-server: "http://127.0.0.1:12340"

  # 下载电台必需，见下文“获取 media-user-token”。
  media-user-token: "your-media-user-token"

# 保存目录。相对路径从运行目录解析。
paths:
  alac: "AM-Lossless"
  atmos: "AM-Atmos"
  aac: "AM-AAC"
  mv: "AM-MV"
```

如果 wrapper-lite 运行在其他机器或容器中，把 `127.0.0.1` 换成该主机的局域网地址或公网可达地址。

## 使用方法

执行任何命令前确认：

1. wrapper-lite 正在运行。
2. `config.yaml` 存在，并且 `lite-server` 正确。

### 专辑

```bash
./amdl "https://music.apple.com/us/album/whenever-you-need-somebody-2022-remaster/1624945511"
```

Windows PowerShell：

```powershell
.\amdl.exe "https://music.apple.com/us/album/whenever-you-need-somebody-2022-remaster/1624945511"
```

### 单曲

```bash
./amdl "https://music.apple.com/us/album/never-gonna-give-you-up-2022-remaster/1624945511?i=1624945512"
./amdl "https://music.apple.com/us/song/you-move-me-2022-remaster/1624945520"
```

### 歌手全部专辑

```bash
./amdl --all-album "https://music.apple.com/us/artist/taylor-swift/159260351"
```

### 播放列表

```bash
./amdl "https://music.apple.com/us/playlist/taylor-swift-essentials/pl.3950454ced8c45a3b0cc693c2a7db97b"
```

### 交互式选择

```bash
./amdl --select "https://music.apple.com/us/album/whenever-you-need-somebody-2022-remaster/1624945511"
```

输入以空格分隔的曲目编号。

### 交互式搜索

```bash
./amdl --search album "never gonna give you up"
./amdl --search song "you move me"
./amdl --search artist "taylor swift"
```

### 杜比全景声

```bash
./amdl --atmos "https://music.apple.com/us/album/1989-taylors-version-deluxe/1713845538"
```

### AAC

```bash
./amdl --aac "https://music.apple.com/us/album/1989-taylors-version-deluxe/1713845538"
```

### 查看音质信息

```bash
./amdl --debug "https://music.apple.com/us/album/1989-taylors-version-deluxe/1713845538"
```

### 无损下载的边界处理

不带 `--aac` / `--atmos` 时，程序只在 ALAC 流中选择，并取**不超过 `media.alac-max` 的最高采样率与位深**（例如同时存在 24bit/192kHz 与 24bit/96kHz 时会选 192kHz）。

两个可选开关改变无 ALAC 流时的行为：

| 配置项 | 默认 | 说明 |
|---|---|---|
| `media.lossless-only` | `false` | 设为 `true` 后，若某曲目没有无损流，则**跳过并计为 Unavailable**，不再静默降级为 AAC。不影响显式指定 `--aac` / `--atmos`。 |
| `media.alac-verify` | `false` | 设为 `true` 后，下载完成时扫描 ALAC 流，报告损坏包数量，并在文件中找不到 ALAC 轨道时报错。仅在 `media.alac-fix` 为 `false` 时生效（两者都会遍历全部分包，为免重复扫描不会同时运行）。 |

```yaml
media:
  alac-max: 192000        # 采样率上限；超过 192000 会被自动收敛到 192000
  alac-fix: true          # 自动修复缺失 TYPE_END 的 ALAC 包（开启时会同时报告损坏数量）
  alac-verify: false      # alac-fix 关闭时，下载后只扫描不修改
  lossless-only: false    # 没有无损流时跳过，而不是降级为 AAC
```

### 常用参数

```text
--alac-max <sample-rate>
--atmos-max <bitrate>
--aac-type <aac|aac-lc|aac-binaural|aac-downmix>
--mv-max <resolution>
--mv-audio-type <atmos|ac3|aac>
--lite-server <wrapper-lite-url>
```

## 获取 media-user-token

`media-user-token` 是下载电台必需的。

1. 打开 [Apple Music](https://music.apple.com) 并登录。
2. 按 `F12` 打开开发者工具。
3. 进入 `Application` > `Storage` > `Cookies` > `https://music.apple.com`。
4. 找到名为 `media-user-token` 的 Cookie，复制它的值。
5. 将值粘贴到 `config.yaml` 的 `general.media-user-token`。
6. 重启下载器。

## 歌词设置

在 `config.yaml` 的 `metadata.lyrics` 节点下配置歌词选项：

```yaml
metadata:
  lyrics:
    save-file: false                      # 是否保存外部歌词文件（.lrc / .ttml）到磁盘
    embed: true                           # 是否将歌词直接嵌入音频文件标签
    type: "lyrics"                        # 歌词类型："lyrics"（标准逐行歌词）或 "syllable-lyrics"（逐词/逐字动态歌词）
    format: "lrc"                         # 歌词格式："lrc" 或 "ttml"
    extra: ""                             # 附加歌词选项：""（默认原语种）、"translation"（翻译歌词）或 "pronunciation"（发音/罗马音歌词）
```

> **提示**：
> - 可在 `config.yaml` 的 `general.language` 中配置目标语言代码（如 `"zh-Hans-CN"`、`"en-US"`、`"ja"` 等），用以控制歌词翻译的目标语言。
> - `extra` 设置为 `"translation"` 获取翻译歌词，设置为 `"pronunciation"` 获取发音/罗马音歌词（需 Apple Music 官方提供对应数据）。

## 升级

### 内置自升级（推荐）

通过内置的自升级命令一键升级至最新官方 Release，自动下载校验对应平台二进制，并提供配置增量迁移向导：

```bash
# 检查并执行自升级与配置迁移
./amdl --update

# 也可以使用简写
./amdl -U

# 仅检查是否有新版本，不执行下载
./amdl --check-update

# 脚本或自动化模式（自动接受新增配置默认值）
./amdl -U -y
```

Windows PowerShell：

```powershell
# 执行自升级
.\amdl.exe --update

# 简写
.\amdl.exe -U
```

> **提示**：自升级会自动创建 `config.yaml.bak_...` 备份文件，并安全引导合并新配置项，原有注释与凭据不会丢失。如果配置了 `proxy`，自升级将自动走代理连接官方 GitHub。

### 手动更新二进制

直接前往 [GitHub Releases](https://github.com/zhaarey/apple-music-downloader/releases/latest) 下载对应平台的最新预编译二进制文件，替换原有的 `amdl` 或 `amdl.exe` 即可。

> 如果是通过源码构建的用户，请参阅 [开发者指南](#开发者指南)。

## 开发者指南

如果你希望参与代码贡献或自行从源码构建：

### 环境要求

1. **Go 1.23.1 或更新版本**：[go.dev/dl](https://go.dev/dl/)。
2. **Git**：[git-scm.com](https://git-scm.com/)。
3. **ffmpeg**：[ffmpeg.org](https://ffmpeg.org/)（可选，用于后处理转换或动态封面）。

### 克隆与构建

**macOS / Linux**：

```bash
git clone https://github.com/zhaarey/apple-music-downloader.git
cd apple-music-downloader
cp config.yaml.example config.yaml
go build -o amdl .
./amdl --help
```

**Windows (PowerShell)**：

```powershell
git clone https://github.com/zhaarey/apple-music-downloader.git
cd apple-music-downloader
copy config.yaml.example config.yaml
go build -o amdl.exe .
.\amdl.exe --help
```

### 开发环境运行

```bash
go run . "https://music.apple.com/us/album/whenever-you-need-somebody-2022-remaster/1624945511"
```

### 源码更新

```bash
git pull
go build -o amdl .
```

Windows (PowerShell)：

```powershell
git pull
go build -o amdl.exe .
```

### 发布版本

版本发布由 `VERSION` 文件驱动：推送到 `main` 时，如果 `v<VERSION>` 这个 tag 还不存在，Release 工作流会构建所有平台的二进制并以该 tag 发布 GitHub Release。发版只需修改 `VERSION`（如 `0.1.0` → `0.1.1`）并合并到 `main`。带后缀的版本（如 `0.2.0-beta.1`）会作为预发布版本发布。

Release 二进制会从构建它的仓库自升级，因此 fork 发布的版本会从 fork 获取更新。

### 本地打包发布产物

`scripts/` 下有与 CI 完全一致的构建脚本，**不需要 GitHub Actions 也能产出可发布的 EXE**（版本号、提交号、构建时间注入方式与 CI 相同）。

**Windows (PowerShell)**：

```powershell
.\scripts\build.ps1                       # 只构建当前平台
.\scripts\build.ps1 -All                  # 构建全部平台 + dist\checksums.txt
.\scripts\build.ps1 -Targets windows/amd64 -Gui   # 窗口化 EXE（无双击黑框）
```

**macOS / Linux**：

```bash
./scripts/build.sh                        # 只构建当前平台
./scripts/build.sh --all                  # 全部平台
./scripts/build.sh --targets windows/amd64,linux/amd64
```

产物位于 `dist/`，命名与 Release 资源一致（如 `amdl_windows_amd64.exe`），并生成 `checksums.txt`：

```bash
sha256sum -c dist/checksums.txt           # 校验
```

验证版本注入是否生效：

```powershell
.\dist\amdl_windows_amd64.exe --version
# amdl v0.1.0 (commit: 4741380..., built at: 2026-10-20T01:57:17Z, windows/amd64, go1.23.1)
```

| 参数 | 说明 |
|---|---|
| `-All` / `--all` | 构建全部平台（Android 需要 NDK，缺失时自动跳过并提示） |
| `-Targets` / `--targets` | 指定 `os/arch` 列表，逗号分隔 |
| `-Version` / `--version` | 覆盖版本号（默认读 `VERSION` 文件） |
| `-Out` / `--out` | 输出目录（默认 `dist`） |
| `-Gui` | **仅 Windows**：构建窗口化 EXE（PE 子系统 = GUI），双击不会闪控制台黑框 |
| `-RepoOwner` / `-RepoName` | 覆盖自升级目标仓库（默认从 `git remote origin` 推断） |

> **`-Gui` 的取舍**：窗口化 EXE 双击体验最好（不会闪一个黑框），但它**没有控制台**，
> 因此无法在终端里看输出，带 URL 的命令行下载也不适用。所以：
> 想要"双击即用"的桌面体验就加 `-Gui`；想让同一个文件既能双击又能当命令行工具用，就用默认构建。
> CI 目前发布的是**默认（控制台）构建**——它同样支持双击打开界面，只是会伴随一个控制台窗口。

## 隐私与安全

本项目需要你的 Apple Music 账号才能工作。**登录凭据与令牌只应留在你自己的机器上，切勿上传或分享。**

### 不要上传的文件

| 文件 / 目录 | 内容 | 为什么不能上传 |
|---|---|---|
| `config.yaml` | `media-user-token`、`authorization-token`、代理地址 | 令牌可直接代表你的账号身份 |
| wrapper-lite 的 `rootfs/data/` | `MUSIC_TOKEN`、`DEV_TOKEN`、`token_cache.json`、`accounts.sqlitedb` 等 | 登录后的会话令牌，可直接复用你的账号 |
| wrapper-lite 的 `data/2fa.txt` | 你输入的 2FA 验证码 | 登录凭据 |
| 终端回显 / 截图 | 登录命令与输出 | 可能包含账号、密码或验证码 |

### 使用时请注意

1. **登录凭据只由你本人输入。** 首次登录时（`wrapper-lite` 的 `--login`）请在本地终端自行输入账号与
   2FA 码，不要把密码写进脚本、聊天记录或提交到仓库。
2. **`--login` 参数会明文出现在进程列表中。** 在 Windows 任务管理器 / `ps` 中可以看到
   `--login 账号:密码`。这是 wrapper-lite 的设计限制，不是本项目的代码问题。
   - 建议：登录完成后（token 已缓存）**修改一次 Apple ID 密码**，日常使用不再需要传凭据。
   - 在共享或多人使用的机器上尤其要注意。
   - 登录完成后请关闭那个终端窗口，避免凭据留在回滚缓冲或会话历史中。
3. **不要公开可访问的 `lite-server`。** 把 wrapper-lite 监听在 `0.0.0.0` 或暴露到公网，
   等于把账号解密能力开放给他人。跨机器使用时请限制在可信内网，并自行做好访问控制。
4. **本项目的仓库已把上述敏感文件排除在版本控制之外。**
   `config.yaml` 未被列入 `.gitignore` 白名单（该文件是白名单模式，只放行源码与文档），
   因此不会被 `git add` 意外提交；wrapper-lite 的状态目录位于本项目仓库之外。
   可用下面命令自行确认：

   ```bash
   git check-ignore -v config.yaml
   # 输出形如：.gitignore:1:*   config.yaml   → 说明已被忽略
   ```
5. **图形界面本身不收集也不上传任何数据。** 界面只监听本机回环地址，请求直接发往
   Apple 与 `lite-server`；下载日志仅保留在内存中（最多 400 行），不落盘、不外传。
6. **本机持久化的只有两类数据，均不含凭据。**
   - `%APPDATA%\amdl\gui.json` —— 固定的监听端口与界面访问令牌（仅本机可用）。
   - `%APPDATA%\amdl\history.json` —— 下载历史元数据（名称、音质、保存目录等）。

   这两处都不包含 Apple ID、密码或媒体令牌。删除它们只会重置界面地址与历史，不影响登录。
   想彻底清理时，除了上述两个文件，还需删除 wrapper-lite 的 `rootfs/data/` 目录。

> 如果你怀疑凭据已经泄露（例如误传到公开仓库、粘贴到聊天窗口），
> 请立即修改 Apple ID 密码，并删除 wrapper-lite 的 `rootfs/data/` 下的令牌文件后重新登录。

## 致谢

- **Sorrow** 编写了原始脚本。
- **WorldObservationLog** 开发了 [wrapper / wrapper-lite](https://github.com/WorldObservationLog/wrapper)，本项目将其作为后端解密服务。
- **Sendy McSenderson** 提供了流式下载和解密实现。
- [go-mp4tag](https://github.com/itouakirai/go-mp4tag) 用于写入电台 MP4 元数据。
- [FFmpeg](https://ffmpeg.org/) 支持可选的转换和动态封面功能。
