## 编译注意事项

### 一. 请务必在mac或linux下执行go mod和modvendor相关操作

windows下执行会导致编译时找不到pg_query.h, 即使是用在wsl下用docker编译也不行

### 二. 请使用make vendor更新依赖

有封装好的make vendor命令, 直接用就行, 主要机器上需要安装modvendor, 安装方式见 https://github.com/goware/modvendor