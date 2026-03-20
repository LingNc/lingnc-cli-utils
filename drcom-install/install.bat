@echo off
setlocal EnableExtensions

net session >nul 2>&1
if not %errorlevel%==0 (
  echo [ERROR] 请以管理员身份运行此脚本。
  exit /b 1
)

echo === Dogcom Windows 自动安装 ===
set /p DOGCOM_USERNAME=请输入校园网账号:
set /p DOGCOM_PASSWORD=请输入校园网密码:

if "%DOGCOM_USERNAME%"=="" (
  echo [ERROR] 账号不能为空。
  exit /b 1
)

if "%DOGCOM_PASSWORD%"=="" (
  echo [ERROR] 密码不能为空。
  exit /b 1
)

powershell -NoProfile -ExecutionPolicy Bypass -File "%~dp0install.ps1" -Username "%DOGCOM_USERNAME%" -Password "%DOGCOM_PASSWORD%"
if not %errorlevel%==0 (
  echo [ERROR] 安装失败，请查看上面的错误输出。
  exit /b 1
)

echo [OK] 安装完成。
echo 查看计划任务: schtasks /Query /TN Dogcom /V /FO LIST
echo 查看日志文件: type C:\ProgramData\Dogcom\dogcom.log
exit /b 0
