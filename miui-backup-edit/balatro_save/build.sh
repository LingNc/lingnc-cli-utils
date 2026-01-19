# 构建win版本
GOOS=windows GOARCH=amd64 go build -o balatro_save.exe .
# 构建linux版本
GOOS=linux GOARCH=amd64 go build -o balatro_save .