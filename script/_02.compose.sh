#用了docker-compose就只需要运行这一个文件了

docker-compose down --rmi all #移除通过compose创建的所有镜像
#--build参数 它适合在您对 Dockerfile 或服务配置进行了更改后使用。
#docker-compose up --build
docker-compose up #依据 docker-compose.yml 文件里的配置来创建并启动所有的服务容器。加参数 -d是后台运行

