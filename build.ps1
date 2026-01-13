Write-Host "编译带图标的角色卡管理器..." -ForegroundColor Green

try {
    # 检查是否安装了rsrc工具
    Write-Host "检查rsrc工具..." -ForegroundColor Yellow
    & go version | Out-Null
    if ($LASTEXITCODE -ne 0) {
        throw "Go未安装或不在PATH中"
    }
    
    # 安装rsrc工具（如果没有的话）
    Write-Host "安装/更新rsrc工具..." -ForegroundColor Yellow
    & go install github.com/akavel/rsrc@latest
    
    # 生成syso文件
    Write-Host "生成Windows资源文件..." -ForegroundColor Yellow
    & rsrc -ico public/ico.ico -o main_windows.syso
    
    if ($LASTEXITCODE -ne 0) {
        throw "rsrc生成失败"
    }
    
    # 编译程序（Go会自动包含syso文件）
    Write-Host "正在编译程序..." -ForegroundColor Yellow
    & go build -o "角色卡管理器.exe" .
    
    if ($LASTEXITCODE -ne 0) {
        throw "编译失败"
    }
    
    Write-Host "✓ 编译成功！生成带图标的文件：角色卡管理器.exe" -ForegroundColor Green
    
} catch {
    Write-Host "编译失败：$($_.Exception.Message)" -ForegroundColor Red
    Write-Host "尝试普通编译..." -ForegroundColor Yellow
    
    # 清理可能的syso文件
    Remove-Item main_windows.syso -ErrorAction SilentlyContinue
    
    # 普通编译
    & go build -o "角色卡管理器.exe" .
    
    if ($LASTEXITCODE -eq 0) {
        Write-Host "✓ 普通编译成功！生成文件：角色卡管理器.exe" -ForegroundColor Green
    } else {
        Write-Host "✗ 编译完全失败！" -ForegroundColor Red
    }
}

# 显示文件信息
if (Test-Path "角色卡管理器.exe") {
    $fileInfo = Get-Item "角色卡管理器.exe"
    Write-Host "文件大小：$([math]::Round($fileInfo.Length / 1MB, 2)) MB" -ForegroundColor Cyan
}

Read-Host "按回车键退出"