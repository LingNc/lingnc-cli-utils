@echo off
setlocal EnableExtensions

net session >nul 2>&1
if not %errorlevel%==0 (
  echo [ERROR] 请以管理员身份运行此脚本。
  exit /b 1
)

powershell -NoProfile -ExecutionPolicy Bypass -File "%~dp0uninstall.ps1"
if not %errorlevel%==0 (
  echo [ERROR] 卸载失败，请查看上面的错误输出。
  exit /b 1
)

echo [OK] 卸载完成。
exit /b 0
