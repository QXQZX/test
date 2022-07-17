---
title: 'k8s系列(二)--使用minikube部署简单的应用'
tags: ["k8s"]
categories: ["k8s"]
date: "2022-07-17T20:02:32+08:00"
toc: true
draft: false
---

minikube是本地的Kubernetes，致力于使Kubernetes易于学习和开发。学习使用minikube记录。

<!--more-->
k8s安装 ：

* https://skyao.io/learning-kubernetes/docs/installation.html
* http://soulmz.me/2020/04/29/minikube-installed-for-mac/
* https://v2as.com/article/fba7b8ff-e3e7-49bd-ab0b-31e517147e91



## 安装minibube

https://minikube.sigs.k8s.io/docs/start/

minikube是本地的Kubernetes，致力于使Kubernetes易于学习和开发。



## 启动创建集群

```bash
$ minikube start --image-mirror-country='cn' --image-repository='registry.cn-hangzhou.aliyuncs.com/google_containers' --alsologtostderr

➜  ~ minikube dashboard
🔌  正在开启 dashboard ...
    ▪ Using image registry.cn-hangzhou.aliyuncs.com/google_containers/metrics-scraper:v1.0.7
    ▪ Using image registry.cn-hangzhou.aliyuncs.com/google_containers/dashboard:v2.3.1
🤔  正在验证 dashboard 运行情况 ...
🚀  Launching proxy ...
🤔  正在验证 proxy 运行状况 ...
🎉  Opening http://127.0.0.1:65043/api/v1/namespaces/kubernetes-dashboard/services/http:kubernetes-dashboard:/proxy/ in your default browser...


```

启动dashboard

```shell
minikube dashboard
```



## 在集群中部署应用

```shell
# minikube 教程如下
kubectl create deployment hello-minikube --image=k8s.gcr.io/echoserver:1.4
kubectl expose deployment hello-minikube --type=NodePort --port=8080
kubectl get services hello-minikube
minikube service hello-minikube
kubectl port-forward service/hello-minikube 7080:8080
# Tada! Your application is now available at http://localhost:7080/.
```

发现并不能访问，嗯，发生了什么？？？我是根据minkube官方文档一步一步来的，没错呀。

上网搜索，寻找问题。发现当pod无法启动的时候，可以通过下面的命令排查。通过`kubectl get pods -o wide`命令，发现处于pod启动卡到了 ImagePullBackOff 状态。通过`kubectl describe pod`命令查看pod 详细描述。

```shell
$ kubectl get pods -o wide
$ kubectl describe pod

➜  ~ kubectl get pods  -o wide
NAME                              READY   STATUS             RESTARTS   AGE     IP           NODE       NOMINATED NODE   READINESS GATES
hello-minikube-7bc9d7884c-rhjsz   0/1     ImagePullBackOff   0          5m40s   172.17.0.5   minikube   <none>           <none>
➜  ~ kubectl describe pod
Name:         hello-minikube-7bc9d7884c-rhjsz
Namespace:    default
Priority:     0
Node:         minikube/192.168.49.2
Start Time:   Sun, 17 Jul 2022 15:36:25 +0800
Labels:       app=hello-minikube
              pod-template-hash=7bc9d7884c
Annotations:  <none>
Status:       Pending
IP:           172.17.0.5
IPs:
  IP:           172.17.0.5
Controlled By:  ReplicaSet/hello-minikube-7bc9d7884c
Containers:
  echoserver:
    Container ID:
    Image:          k8s.gcr.io/echoserver:1.4
    Image ID:
    Port:           <none>
    Host Port:      <none>
    State:          Waiting
      Reason:       ImagePullBackOff
    Ready:          False
    Restart Count:  0
    Environment:    <none>
    Mounts:
      /var/run/secrets/kubernetes.io/serviceaccount from kube-api-access-bmh6t (ro)
Conditions:
  Type              Status
  Initialized       True
  Ready             False
  ContainersReady   False
  PodScheduled      True
Volumes:
  kube-api-access-bmh6t:
    Type:                    Projected (a volume that contains injected data from multiple sources)
    TokenExpirationSeconds:  3607
    ConfigMapName:           kube-root-ca.crt
    ConfigMapOptional:       <nil>
    DownwardAPI:             true
QoS Class:                   BestEffort
Node-Selectors:              <none>
Tolerations:                 node.kubernetes.io/not-ready:NoExecute op=Exists for 300s
                             node.kubernetes.io/unreachable:NoExecute op=Exists for 300s
Events:
  Type     Reason     Age                    From               Message
  ----     ------     ----                   ----               -------
  Normal   Scheduled  5m50s                  default-scheduler  Successfully assigned default/hello-minikube-7bc9d7884c-rhjsz to minikube
  Normal   Pulling    3m42s (x4 over 5m50s)  kubelet            Pulling image "k8s.gcr.io/echoserver:1.4"
  Warning  Failed     3m27s (x4 over 5m34s)  kubelet            Failed to pull image "k8s.gcr.io/echoserver:1.4": rpc error: code = Unknown desc = Error response from daemon: Get "https://k8s.gcr.io/v2/": net/http: request canceled while waiting for connection (Client.Timeout exceeded while awaiting headers)
  Warning  Failed     3m27s (x4 over 5m34s)  kubelet            Error: ErrImagePull
  Warning  Failed     3m14s (x6 over 5m34s)  kubelet            Error: ImagePullBackOff
  Normal   BackOff    48s (x15 over 5m34s)   kubelet            Back-off pulling image "k8s.gcr.io/echoserver:1.4"
```

镜像拉取失败了，大概知道问题出在哪了，gfw的锅。

于是经过了一番摸索，找到了几种解决方式。下面我用了比较简单的一种方式尝试了一下，成功解决了这个问题。具体方式如下：

在需要用到k8s.gcr.io镜像的地方，都统一切换到 `registry.cn-hangzhou.aliyuncs.com/google_containers`。

```shell
# 先删掉失败的deployment
$ kubectl delete deployment hello-minikube

# 替换原来的镜像到阿里镜像仓库
# kubectl create deployment hello-minikube --image=k8s.gcr.io/echoserver:1.4
$ kubectl create deployment hello-minikube --image=registry.cn-hangzhou.aliyuncs.com/google_containers/echoserver:1.4
$ kubectl port-forward service/hello-minikube-2 7080:8080
```

访问127.0.0.1:7080，浏览器显示如下，deployment创建成功。

```
CLIENT VALUES:
client_address=127.0.0.1
command=GET
real path=/
query=nil
request_version=1.1
request_uri=http://127.0.0.1:8080/

SERVER VALUES:
server_version=nginx: 1.10.0 - lua: 10001

HEADERS RECEIVED:
accept=text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.9
accept-encoding=gzip, deflate, br
accept-language=zh-CN,zh;q=0.9
cache-control=max-age=0
connection=keep-alive
dnt=1
host=127.0.0.1:7080
sec-ch-ua=".Not/A)Brand";v="99", "Google Chrome";v="103", "Chromium";v="103"
sec-ch-ua-mobile=?0
sec-ch-ua-platform="macOS"
sec-fetch-dest=document
sec-fetch-mode=navigate
sec-fetch-site=none
sec-fetch-user=?1
sec-gpc=1
upgrade-insecure-requests=1
user-agent=Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/103.0.0.0 Safari/537.36
BODY:
-no body in request-
```

新建终端，通过`minikube dashboard`命令可以查看，deployment、service、pod信息

![image-20220717212851546](https://image-ihui.oss-cn-beijing.aliyuncs.com/img/image-20220717212851546.png)

当然这里还有其他很多的方式来解决镜像拉取失败而导致的pod启动失败的问题。



### 解决k8s部署镜像拉取失败的其他方式

#### 1. 替换镜像tag

k8s.gcr.io/echoserver:1.4  ==>   registry.cn-hangzhou.aliyuncs.com/google_containers/echoserver:1.4

#### 2. 更换代理

如果你已经在本地windows、mac上使用 `某种不可名状` 的工具，默认可以通过它的1080、1087等端口来拉取镜像。

linux

```shell
sudo mkdir -p /etc/systemd/system/docker.service.d 
sudo touch /etc/systemd/system/docker.service.d/proxy.conf
sudo chmod 777 /etc/systemd/system/docker.service.d/proxy.conf
sudo echo '
[Service]
Environment="HTTP_PROXY=http://127.0.0.1:1087"
Environment="HTTPS_PROXY=http://127.0.0.1:1087"
' >> /etc/systemd/system/docker.service.d/proxy.conf
sudo systemctl daemon-reload
sudo systemctl restart docker
sudo systemctl restart kubelet
```

mac

下面改成你代理工具的端口！

![image-20220717214053161](https://image-ihui.oss-cn-beijing.aliyuncs.com/img/image-20220717214053161.png)

#### 3. 重新打tag

```bash
# 示例
docker pull k8s.gcr.io/echoserver:1.4
# 改为
docker pull registry.cn-hangzhou.aliyuncs.com/google_containers/echoserver:1.4

docker tag registry.cn-hangzhou.aliyuncs.com/google_containers/echoserver:1.4 k8s.gcr.io/echoserver:1.4
```

当然，你也可以在dockerhub选择其他的镜像。在拉取完docker的镜像后，通过命令重新打tag。比如我们可以看看gotok8s有一些镜像，它们应该是就是k8s.gcr.io的国内镜像版本了。

![image-20220717214718678](https://image-ihui.oss-cn-beijing.aliyuncs.com/img/image-20220717214718678.png)



参考链接

* https://blog.csdn.net/networken/article/details/84571373
* https://wxrbwran.github.io/2021/11/02/Docker-Pull%E8%AE%BE%E7%BD%AE%E4%BB%A3%E7%90%86%E8%A7%A3%E5%86%B3Get-https-k8s-gcr-io-v2-net-http-request-canceled-while-waiting-for-connection/



## 其他命令

管理集群的命令

```bash
# 查看部署状态
$ kubectl get deployment

# 查看创建的服务
$ kubectl get services xxxxxx
➜  ~ kubectl get services
NAME               TYPE        CLUSTER-IP      EXTERNAL-IP   PORT(S)          AGE
hello-minikube-2   NodePort    10.104.103.21   <none>        8080:30633/TCP   5h45m
kubernetes         ClusterIP   10.96.0.1       <none>        443/TCP          123d
➜  ~ kubectl get services hello-minikube-2
NAME               TYPE       CLUSTER-IP      EXTERNAL-IP   PORT(S)          AGE
hello-minikube-2   NodePort   10.104.103.21   <none>        8080:30633/TCP   5h46m

# 设置端口转发
$ kubectl port-forward service/hello-minikube-2 7080:8080


# minikube 的一些命令

# 暂停集群而不影响部署的应用程序
minikube pause 

# 停止集群，相对应的是minikube start启动集群
minikube stop 

# 设置内存大小为2048MB，设置后创建新的集群会使用改配置
minikube config set memory 2048 

# 查看扩展插件信息
minikube addons list 

# 删除集群
minikube delete 
```