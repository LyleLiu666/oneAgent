# FastGPT Top Issues (Open & High Reactions)

Fetched at: Fri Jan 30 22:32:29 CST 2026
Repo: https://github.com/labring/FastGPT

## [导入配置-AI对话节点-AI模型变量引用数据丢失](https://github.com/labring/FastGPT/issues/6351) (#6351)
**Created**: 2026-01-30T07:22:01Z

**例行检查**

[//]: # '方框内填 x 表示打钩'

- [x] 我已确认目前没有类似 issue
- [x] 我已完整查看过项目 README，以及[项目文档](https://doc.fastgpt.io/docs/introduction/)
- [x] 我使用了自己的 key，并确认我的 key 是可正常使用的
- [x] 我理解并愿意跟进此 issue，协助测试和提供反馈
- [x] 我理解并认可上述内容，并理解项目维护者精力有限，**不遵循规则的 issue 可能会被无视或直接关闭**

**你的版本**
官方线上版本：https://cloud.fastgpt.cn/
- [x] 公有云版本
- [ ] 私有部署版本, 具体版本号: 

**问题描述, 日志截图，配置文件等**
工作流AI对话节点，AI模型使用变量引用设置，设置完成后对当前工作流导出后再导入，AI模型变量引用的数据会丢失

**复现步骤**
1、创建一个工作流，添加一个全局变量 ModuleName，类型设置为内部变量，值设置为任意值（如123456）
2、添加一个AI对话节点
3、在AI对话节点-->输入-->AI模型，点击“变量引用”，在对应的select下拉框中选择“全局变量>ModuleName”
4、保存变发布
5、点击左上角menu icon-->导出配置--> 复制到剪贴板
6、点击左上角menu icon-->导入配置-->在弹出框中粘贴内容-->保存
7、AI对话节点-->AI模型，“变量引用”值为空，已经丢失
**预期结果**
数据不丢失，导出后再导入，数据配置不丢失，保持一致
**相关截图**
如图所示
初始配置页面：

<img width="1402" height="838" alt="Image" src="https://github.com/user-attachments/assets/285b202f-67c0-479c-95ed-e8aa84fd8b5b" />
导入后结果页面：

<img width="1246" height="863" alt="Image" src="https://github.com/user-attachments/assets/8a863fd0-a77f-4a26-9e28-99d82a581c0d" />

---

## [iPhone Safari 首次打开免登录分享链接报错"凭证已过期，请重新登录"](https://github.com/labring/FastGPT/issues/6335) (#6335)
**Created**: 2026-01-29T02:37:40Z

**例行检查**

[//]: # '方框内填 x 表示打钩'

- [x] 我已确认目前没有类似 issue
- [x] 我已完整查看过项目 README，以及[项目文档](https://doc.fastgpt.io/docs/introduction/)
- [x] 我使用了自己的 key，并确认我的 key 是可正常使用的
- [x] 我理解并愿意跟进此 issue，协助测试和提供反馈
- [x] 我理解并认可上述内容，并理解项目维护者精力有限，**不遵循规则的 issue 可能会被无视或直接关闭**

**你的版本**

- [ ] 公有云版本
- [ ] 私有部署版本, 具体版本号: v4.14.5.1 

**问题描述, 日志截图，配置文件等**

在 iPhone Safari 浏览器上，首次直接打开免登录分享链接（/chat/share?shareId=xxx）时会报错"凭证已过期，请重新登录"，但刷新页面后恢复正常。同样的链接在电脑 Chrome 浏览器上首次打开正常。

**复现步骤**
- 创建一个应用的免登录分享链接
- 在 iPhone Safari 浏览器中首次直接打开该链接（非隐私模式）
- 页面显示错误提示："凭证已过期，请重新登录"
- 刷新页面后，问题消失，可以正常使用

**预期结果**

首次打开免登录分享链接应该正常加载，无需刷新。

**相关截图**

<img width="468" height="915" alt="Image" src="https://github.com/user-attachments/assets/2a7b87de-ed19-472e-82aa-52cd9b79fbf7" />

**可能的原因**
经过代码分析，这是一个 zustand hydration 时序问题：
- share.tsx 中的 localUId 存储在 useShareChatStore（使用 zustand persist + localStorage）
- 首次访问时，zustand 需要从 localStorage hydrate 数据，此时 loaded = false，localUId = undefined
- 但 ChatContextProvider 中的 useScrollPagination 在组件挂载时会立即发起请求（manual: false）
- 此时 outLinkUid 参数为空，导致服务端返回 403 错误
- 前端将 403 错误码识别为 TOKEN_ERROR_CODE，显示"凭证已过期"

---

## [使用多模态大模型API，上传图片之后有时不能识别到图片](https://github.com/labring/FastGPT/issues/6332) (#6332)
**Created**: 2026-01-28T02:19:46Z

**例行检查**

[//]: # '方框内填 x 表示打钩'

- [x ] 我已确认目前没有类似 issue
- [ x] 我已完整查看过项目 README，以及[项目文档](https://doc.fastgpt.io/docs/introduction/)
- [x ] 我使用了自己的 key，并确认我的 key 是可正常使用的
- [ x] 我理解并愿意跟进此 issue，协助测试和提供反馈
- [x] 我理解并认可上述内容，并理解项目维护者精力有限，**不遵循规则的 issue 可能会被无视或直接关闭**

**你的版本**

- [ ] 公有云版本
- [x ] 私有部署版本, 具体版本号: 4.13.2

**问题描述, 日志截图，配置文件等**

通过API的方式去识别无法找到图片

<img width="1449" height="814" alt="Image" src="https://github.com/user-attachments/assets/78346c2f-cdf1-495a-a6c1-c02fc8415916" />

<img width="596" height="561" alt="Image" src="https://github.com/user-attachments/assets/69a6cad3-ca67-44d5-ba48-690457e16220" />

正常调试是可以的：

<img width="996" height="804" alt="Image" src="https://github.com/user-attachments/assets/14e1c8fc-63b6-4ca2-8826-4398524420dd" />

**复现步骤**
使用Api进行图片识别

**预期结果**

**相关截图**

<img width="641" height="669" alt="Image" src="https://github.com/user-attachments/assets/c4e8d214-f529-47bf-8904-9cb890bc185a" />

---

## [后续会引进Skills功能么](https://github.com/labring/FastGPT/issues/6320) (#6320)
**Created**: 2026-01-26T08:51:08Z

**例行检查**

[//]: # '方框内填 x 表示打钩'

- [ ] 我已确认目前没有类似 features
- [ ] 我已确认我已升级到最新版本
- [ ] 我已完整查看过项目 README，已确定现有版本无法满足需求
- [ ] 我理解并愿意跟进此 features，协助测试和提供反馈
- [x] 我理解并认可上述内容，并理解项目维护者精力有限，**不遵循规则的 features 可能会被无视或直接关闭**

**功能描述**
工具中新增导入Skills的功能，除了“简易模式”、“workflow”外，加一套Skills的方式。
**应用场景**

**相关示例**


---

## [上传文件卡在进度17，页面提示net::ERR_CONNECTION_RESET](https://github.com/labring/FastGPT/issues/6319) (#6319)
**Created**: 2026-01-26T07:40:04Z

**例行检查**

[//]: # '方框内填 x 表示打钩'

- [x] 我已确认目前没有类似 issue
- [x] 我已完整查看过项目 README，以及[项目文档](https://doc.fastgpt.io/docs/introduction/)
- [x] 我使用了自己的 key，并确认我的 key 是可正常使用的
- [x] 我理解并愿意跟进此 issue，协助测试和提供反馈
- [x] 我理解并认可上述内容，并理解项目维护者精力有限，**不遵循规则的 issue 可能会被无视或直接关闭**

**你的版本**

- [ ] 公有云版本
- [x] 私有部署版本, 具体版本号: v4.14.5.1

**问题描述, 日志截图，配置文件等**
问题：新建知识库上传文件，进度卡住，如下图：

<img width="2413" height="1293" alt="Image" src="https://github.com/user-attachments/assets/f0606b5b-68f1-46f7-b2ca-0eaaa18f1bc4" />

plugin日志正常：

<img width="1917" height="960" alt="Image" src="https://github.com/user-attachments/assets/10b4685c-6a18-4096-89c5-933eb6df57e1" />

容器开放如图所示：

<img width="1916" height="243" alt="Image" src="https://github.com/user-attachments/assets/1d1c1cf4-ae87-43fb-b525-de17d8968e4e" />

配置如下：
`# 用于部署的 docker-compose 文件:
# - FastGPT 端口映射为 3000:3000
# - FastGPT-mcp-server 端口映射 3005:3000
# - 建议修改账密后再运行

# plugin auth token
x-plugin-auth-token: &x-plugin-auth-token 'token'
# aiproxy token
x-aiproxy-token: &x-aiproxy-token 'token'
# 数据库连接相关配置
x-share-db-config: &x-share-db-config
  MONGODB_URI: mongodb://myusername:mypassword@mongo:27017/fastgpt?authSource=admin
  DB_MAX_LINK: 100
  REDIS_URL: redis://default:mypassword@redis:6379
  # @see https://fastgpt.cn/docs/introduction/development/object-storage
  STORAGE_VENDOR: minio # minio | aws-s3 | cos | oss
  STORAGE_REGION: us-east-1
  STORAGE_ACCESS_KEY_ID: minioadmin
  STORAGE_SECRET_ACCESS_KEY: minioadmin
  STORAGE_PUBLIC_BUCKET: fastgpt-public
  STORAGE_PRIVATE_BUCKET: fastgpt-private
  STORAGE_EXTERNAL_ENDPOINT: http://172.18.60.60:9000 # 一个服务器和客户端均可访问到存储桶的地址，可以是固定的宿主机 IP 或者域名，注意不要填写成 127.0.0.1 或者 localhost 等本地回环地址(因为容器里无法使用)
  STORAGE_S3_ENDPOINT: http://fastgpt-minio:9000 # 协议://域名(IP):端口
  STORAGE_S3_FORCE_PATH_STYLE: true
  STORAGE_S3_MAX_RETRIES: 3

# 向量库相关配置
x-vec-config: &x-vec-config
  PG_URL: postgresql://username:password@vectorDB:5432/postgres

version: '3.3'
services:
  # Vector DB
  vectorDB:
    image: registry.cn-hangzhou.aliyuncs.com/fastgpt/pgvector:0.8.0-pg15
    # container_name: pg
    restart: always
    networks:
      - fastgpt
    environment:
      # 这里的配置只有首次运行生效。修改后，重启镜像是不会生效的。需要把持久化数据删除再重启，才有效果
      - POSTGRES_USER=username
      - POSTGRES_PASSWORD=password
      - POSTGRES_DB=postgres
    volumes:
      - ./pg/data:/var/lib/postgresql/data
    healthcheck:
      test: ['CMD', 'pg_isready', '-U', 'username', '-d', 'postgres']
      interval: 5s
      timeout: 5s
      retries: 10


  mongo:
    image: registry.cn-hangzhou.aliyuncs.com/fastgpt/mongo:5.0.32 # cpu 不支持 AVX 时候使用 4.4.29
    # container_name: mongo
    restart: always
    networks:
      - fastgpt
    command: mongod --keyFile /data/mongodb.key --replSet rs0
    environment:
      - MONGO_INITDB_ROOT_USERNAME=myusername
      - MONGO_INITDB_ROOT_PASSWORD=mypassword
    volumes:
      - ./mongo/data:/data/db
    healthcheck:
      test: ['CMD', 'mongo', '-u', 'myusername', '-p', 'mypassword', '--authenticationDatabase', 'admin', '--eval', "db.adminCommand('ping')"]
      interval: 10s
      timeout: 5s
      retries: 5
      start_period: 30s
    entrypoint:
      - bash
      - -c
      - |
        openssl rand -base64 128 > /data/mongodb.key
        chmod 400 /data/mongodb.key
        chown 999:999 /data/mongodb.key
        echo 'const isInited = rs.status().ok === 1
        if(!isInited){
          rs.initiate({
              _id: "rs0",
              members: [
                  { _id: 0, host: "mongo:27017" }
              ]
          })
        }' > /data/initReplicaSet.js
        # 启动MongoDB服务
        exec docker-entrypoint.sh "$$@" &

        # 等待MongoDB服务启动
        until mongo -u myusername -p mypassword --authenticationDatabase admin --eval "print('waited for connection')"; do
          echo "Waiting for MongoDB to start..."
          sleep 2
        done

        # 执行初始化副本集的脚本
        mongo -u myusername -p mypassword --authenticationDatabase admin /data/initReplicaSet.js

        # 等待docker-entrypoint.sh脚本执行的MongoDB服务进程
        wait $$!
  redis:
    image: registry.cn-hangzhou.aliyuncs.com/fastgpt/redis:7.2-alpine
    # container_name: redis
    networks:
      - fastgpt
    restart: always
    command: |
      redis-server --requirepass mypassword --loglevel warning --maxclients 10000 --appendonly yes --save 60 10 --maxmemory 4gb --maxmemory-policy noeviction
    healthcheck:
      test: ['CMD', 'redis-cli', '-a', 'mypassword', 'ping']
      interval: 10s
      timeout: 3s
      retries: 3
      start_period: 30s
    volumes:
      - ./redis/data:/data
  fastgpt-minio:
    image: registry.cn-hangzhou.aliyuncs.com/fastgpt/minio:RELEASE.2025-09-07T16-13-09Z
    # container_name: fastgpt-minio
    restart: always
    ports:
      - 9002:9000
      - 9003:9001
    networks:
      - fastgpt
    environment:
      - MINIO_ROOT_USER=minioadmin
      - MINIO_ROOT_PASSWORD=minioadmin
    volumes:
      - ./fastgpt-minio:/data
    command: server /data --console-address ":9001"
    healthcheck:
      test: ['CMD', 'curl', '-f', 'http://localhost:9000/minio/health/live']
      interval: 30s
      timeout: 20s
      retries: 3

  fastgpt:
    # container_name: fastgpt
    image: registry.cn-hangzhou.aliyuncs.com/fastgpt/fastgpt:v4.14.5.1 # git
    ports:
      - 3006:3000
    networks:
      - fastgpt
    depends_on:
      - mongo
      - sandbox
      - vectorDB
    restart: always
    environment:
      <<: [*x-share-db-config, *x-vec-config]
      # 前端外部可访问的地址，用于自动补全文件资源路径。例如 https:fastgpt.cn，不能填 localhost。这个值可以不填，不填则发给模型的图片会是一个相对路径，而不是全路径，模型可能伪造Host。
      FE_DOMAIN:
      # root 密码，用户名为: root。如果需要修改 root 密码，直接修改这个环境变量，并重启即可。
      DEFAULT_ROOT_PSW: 1234
      # 登录凭证密钥
      TOKEN_KEY: any
      # root的密钥，常用于升级时候的初始化请求
      ROOT_KEY: root_key
      # 文件阅读加密
      FILE_TOKEN_KEY: filetoken
      # 密钥加密key
      AES256_SECRET_KEY: fastgptkey

      # plugin 地址
      PLUGIN_BASE_URL: http://fastgpt-plugin:3000
      PLUGIN_TOKEN: *x-plugin-auth-token
      # sandbox 地址
      SANDBOX_URL: http://sandbox:3000
      # AI Proxy 的地址，如果配了该地址，优先使用
      AIPROXY_API_ENDPOINT: http://aiproxy:3000
      # AI Proxy 的 Admin Token，与 AI Proxy 中的环境变量 ADMIN_KEY
      AIPROXY_API_TOKEN: *x-aiproxy-token

      # 日志等级: debug, info, warn, error
      LOG_LEVEL: info
      STORE_LOG_LEVEL: warn
      # 工作流最大运行次数
      WORKFLOW_MAX_RUN_TIMES: 1000
      # 批量执行节点，最大输入长度
      WORKFLOW_MAX_LOOP_TIMES: 100
      # 对话文件过期天数
      CHAT_FILE_EXPIRE_TIME: 7
      # 服务器接收请求，最大大小，单位 MB
      SERVICE_REQUEST_MAX_CONTENT_LENGTH: 10
      # HTML 转换最大字符数
      MAX_HTML_TRANSFORM_CHARS: 1000000
    volumes:
      - ./config.json:/app/data/config.json
  sandbox:
    # container_name: sandbox
    image: registry.cn-hangzhou.aliyuncs.com/fastgpt/fastgpt-sandbox:v4.14.5.1
    networks:
      - fastgpt
    restart: always
  fastgpt-mcp-server:
    # container_name: fastgpt-mcp-server
    image: registry.cn-hangzhou.aliyuncs.com/fastgpt/fastgpt-mcp_server:v4.14.5.1
    networks:
      - fastgpt
    ports:
      - 3005:3000
    restart: always
    environment:
      - FASTGPT_ENDPOINT=http://fastgpt:3000
  fastgpt-plugin:
    image: registry.cn-hangzhou.aliyuncs.com/fastgpt/fastgpt-plugin:v0.4.0
    # container_name: fastgpt-plugin
    restart: always
    networks:
      - fastgpt
    environment:
      <<: *x-share-db-config
      AUTH_TOKEN: *x-plugin-auth-token
      # 工具网络请求，最大请求和响应体
      SERVICE_REQUEST_MAX_CONTENT_LENGTH: 10
      # 最大 API 请求体大小
      MAX_API_SIZE: 10
    depends_on:
      fastgpt-minio:
        condition: service_healthy
  # AI Proxy
  aiproxy:
    image: registry.cn-hangzhou.aliyuncs.com/labring/aiproxy:v0.3.2
    # container_name: aiproxy
    restart: unless-stopped
    depends_on:
      aiproxy_pg:
        condition: service_healthy
    networks:
      - fastgpt
      - aiproxy
    environment:
      # 对应 fastgpt 里的AIPROXY_API_TOKEN
      ADMIN_KEY: *x-aiproxy-token
      # 错误日志详情保存时间（小时）
      LOG_DETAIL_STORAGE_HOURS: 1
      # 数据库连接地址
      SQL_DSN: postgres://postgres:aiproxy@aiproxy_pg:5432/aiproxy
      # 最大重试次数
      RETRY_TIMES: 3
      # 不需要计费
      BILLING_ENABLED: false
      # 不需要严格检测模型
      DISABLE_MODEL_CONFIG: true
    healthcheck:
      test: ['CMD', 'curl', '-f', 'http://localhost:3000/api/status']
      interval: 5s
      timeout: 5s
      retries: 10
  aiproxy_pg:
    image: registry.cn-hangzhou.aliyuncs.com/fastgpt/pgvector:0.8.0-pg15 # docker hub
    restart: unless-stopped
    # container_name: aiproxy_pg
    volumes:
      - ./aiproxy_pg:/var/lib/postgresql/data
    networks:
      - aiproxy
    environment:
      TZ: Asia/Shanghai
      POSTGRES_USER: postgres
      POSTGRES_DB: aiproxy
      POSTGRES_PASSWORD: aiproxy
    healthcheck:
      test: ['CMD', 'pg_isready', '-U', 'postgres', '-d', 'aiproxy']
      interval: 5s
      timeout: 5s
      retries: 10
networks:
  fastgpt:
  aiproxy:
  vector:
`

请问是哪里配置还有问题么？

---

