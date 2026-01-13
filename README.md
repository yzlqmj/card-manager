# 酒馆角色卡管理器

这是一个用于管理和组织 [Tavern AI](https://github.com/TavernAI/TavernAI) 和 [SillyTavern](https://github.com/SillyTavern/SillyTavern) 角色卡的桌面工具。它提供了一个简单易用的 Web 界面，帮助您浏览、分类、下载和维护您的角色卡收藏。

## 主要功能

- **角色卡管理**: 以分类和角色的形式清晰地组织您的角色卡。
- **版本控制**: 同一个角色可以保存多个版本的卡片，并轻松切换和预览。
- **导入状态检查**: 自动扫描您的 Tavern/SillyTavern 目录，并显示哪些角色卡已经导入，以及是否为最新版本。
- **一键下载**: 从链接直接下载新的角色卡到指定的分类和角色目录。
- **卡面管理**: 支持下载和预览与角色关联的卡面图片。
- **剪贴板监听**: 自动捕获剪贴板中的 Discord 图片链接，方便快速下载卡面。
- **备注功能**: 为每个角色添加 Markdown 格式的备注。
- **本地化支持**: 集成了本地化工具，可以一键处理需要翻译的角色卡。
- **待整理区**: 将未分类的卡片统一放置在“待整理”区域，方便后续归档。

## 如何编译和运行

### 先决条件

- [Go](https://golang.org/) (建议版本 1.18 或更高)

### 编译

1.  克隆或下载本仓库到您的本地计算机。
2.  打开终端或命令行，进入项目根目录。
3.  运行以下命令来编译生成可执行文件：

    ```bash
    go build
    ```

    这将在项目根目录下生成一个名为 `角色卡管理器.exe` (Windows) 或 `角色卡管理器` (macOS/Linux) 的可执行文件。

### 配置

在首次运行程序之前，您需要配置程序以指向您的角色卡和 Tavern/SillyTavern 目录。

1.  找到 `config` 目录下的 `config.yaml` 文件。
2.  根据您的实际路径修改以下字段：
    - `角色卡根目录`: 您存放角色卡的根目录。程序将在此目录下创建分类和角色文件夹。
    - `酒馆角色卡目录`: Tavern 或 SillyTavern 的 `characters` 目录路径。
    - `酒馆公共目录`: Tavern 或 SillyTavern 的 `public` 目录路径 (用于本地化检查)。
    - `代理地址` (可选): 如果您需要通过代理下载角色卡，请设置代理服务器地址。
    - `端口` (可选): 设置程序运行的端口号，默认为 `3600`。
    - `本地化工具`: 配置本地化工具的相关参数，包括基础路径和强制代理列表。

#### 配置文件示例

```yaml
# 卡片管理器统一配置文件
# 本配置文件包含主应用和本地化工具的所有配置参数

# 角色卡根目录 - 存放所有角色卡文件的主目录
角色卡根目录: "D:\\yw\\AI\\角色卡"

# 酒馆角色卡目录 - SillyTavern应用中角色卡的存储位置
酒馆角色卡目录: "D:\\Software\\AI\\SillyTavern\\SillyTavern\\data\\default-user\\characters"

# 酒馆公共目录 - SillyTavern的公共资源目录
酒馆公共目录: "D:\\Software\\AI\\SillyTavern\\SillyTavern\\public"

# 代理地址 - 网络请求使用的代理服务器地址
代理地址: "http://127.0.0.1:1233"

# 端口 - 应用程序监听的端口号
端口: 3600

# 本地化工具配置
本地化工具:
  # 基础路径 - 本地化资源的基础存储路径
  基础路径: "D:\\Software\\AI\\SillyTavern\\SillyTavern\\public"
  
  # 强制代理列表 - 必须通过代理访问的域名列表
  强制代理列表:
    - "gitgud.io"
    - "raw.githubusercontent.com"
    - "cdn.jsdelivr.net"
    - "github.com"
    - "fonts.googleapis.com"
    - "files.catbox.moe"
```

### 运行

双击运行编译好的可执行文件，或者在终端中执行：

```bash
./角色卡管理器
```

程序启动后，您会看到类似以下的输出：

```
Go server running at http://localhost:3600
访问 http://localhost:3600/index.html 来使用管理器。
```

现在，您可以在浏览器中打开 `http://localhost:3600` 来开始使用角色卡管理器了。