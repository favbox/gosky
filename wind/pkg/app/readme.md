# 应用层 Application

应用层提供了一些通用能力，包括**请求绑定**、**响应渲染**、**服务发现/注册/负载均衡以及服务治理**等等。

其中，**洋葱模型中间件**的核心目的是让业务开发同学**基于这个中间件快速地给业务逻辑进行扩展**，
扩展方式是可以在业务逻辑处理前和处理后分别插桩埋点做相应的处理。一些代表性应用包括：
日志打点、前置的安全检测，都是通过洋葱模型中间件进行处理的。