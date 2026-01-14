# 🎭 酒馆角色卡管理器

<div align="center">

一个现代化的角色卡管理工具，专为 [Tavern AI](https://github.com/TavernAI/TavernAI) 和 [SillyTavern](https://github.com/SillyTavern/SillyTavern) 用户设计

[![Go Version](https://img.shields.io/badge/Go-1.23+-00ADD8?style=flat&logo=go)](https://golang.org/)
[![License](https://img.shields.io/badge/License-GPL-blue.svg)](LICENSE)
[![Platform](https://img.shields.io/badge/Platform-Windows%20%7C%20macOS%20%7C%20Linux-lightgrey)](https://github.com)

</div>

## ✨ 特性

- 🗂️ **智能分类管理** - 按分类和角色清晰组织您的角色卡收藏
- 📦 **版本控制** - 同一角色支持多版本管理，轻松切换预览，支持删除特定版本
- 🔍 **导入状态检查** - 实时扫描 Tavern 目录，显示导入状态和版本信息
- ⬇️ **一键下载** - 从链接直接下载角色卡到指定目录
- 🖼️ **卡面管理** - 下载和预览角色关联的卡面图片
- 📋 **剪贴板监听** - 自动捕获 Discord 图片链接，快速下载
- 📝 **Markdown 备注** - 为每个角色添加丰富的备注信息
- 🌐 **本地化支持** - 集成翻译工具，一键处理多语言角色卡
- 📁 **文件夹操作** - 支持直接在系统文件管理器中打开角色目录
- 🧹 **待整理区管理** - 统一管理未分类卡片，支持整理归档、删除无效文件
- 🔄 **格式转换** - 支持将 JSON 格式角色卡合并为 PNG 格式
- 📊 **统计信息** - 概览收藏总数、待本地化数量等关键指标

## 🚀 快速开始

### 环境要求

- [Go 1.23+](https://golang.org/dl/)

### 安装运行

```bash
# 克隆项目
git clone <repository-url>
cd card-manager

# 编译
go build

# 运行
./card-manager
```

程序将在 `http://localhost:3600` 启动 Web 界面。

## ⚙️ 配置

编辑 `config/config.yaml` 文件（如果不存在则从 `config/config.json` 迁移）：

```yaml
# 角色卡根目录
角色卡根目录: "D:/AI/角色卡"

# SillyTavern 角色卡目录
酒馆角色卡目录: "D:/SillyTavern/data/default-user/characters"

# SillyTavern 公共目录
酒馆公共目录: "D:/SillyTavern/public"

# 代理设置（可选）
代理地址: "http://127.0.0.1:1233"

# 服务端口
端口: 3600

# 本地化工具配置
本地化工具:
  # 本地化资源的基础存储路径
  基础路径: "./localizer_data"
  # 强制使用代理的域名列表
  强制代理列表:
    - "gitgud.io"
    - "raw.githubusercontent.com"
    - "cdn.jsdelivr.net"
```

## 🎯 使用方法

1. **首次启动** - 配置好路径后启动程序
2. **扫描变更** - 点击"扫描变更"按钮同步角色卡状态
3. **下载角色卡** - 使用"下载新卡"功能从链接添加角色，或复制 Discord 链接自动触发下载
4. **管理分类** - 通过拖拽或移动功能整理角色卡
5. **本地化** - 使用内置工具处理多语言角色卡
6. **格式转换** - 在文件列表中将 JSON 卡片转换为 PNG 卡片
7. **清理维护** - 在待整理区检查并归档新文件，删除冗余版本

## 🏗️ 项目结构

```
card-manager/
├── cmd/               # 命令行工具
├── config/            # 配置文件加载与定义
├── internal/          # 核心业务逻辑
│   ├── app/           # 应用初始化与路由
│   ├── config/        # 配置加载逻辑
│   ├── handlers/      # HTTP 处理器 (Cards, Files, System, Tavern)
│   ├── models/        # 数据模型定义
│   └── pkg/           # 工具包 (Cache, Clipboard, Localization, PNG, Tavern)
├── localizer/         # 本地化工具相关
├── public/            # Web 前端资源 (HTML, CSS, JS)
└── main.go            # 程序入口
```

## 🤝 贡献

欢迎提交 Issue 和 Pull Request！

## 📄 许可证

本项目采用 GPL 许可证 - 查看 [LICENSE](LICENSE) 文件了解详情。