# SNMP Agent 库

基于 [GoSNMPServer](https://github.com/slayercat/GoSNMPServer) 封装的 SNMP Agent 库，提供简洁的 API 和完整的日志审计功能。

## 特性

- ✅ **简单易用**：链式 API，快速注册 OID
- ✅ **企业 OID 支持**：自动生成企业前缀 (1.3.6.1.4.1.{PEN})
- ✅ **动态值处理**：支持实时计算的动态值
- ✅ **静态值注册**：快速注册固定值
- ✅ **日志审计**：使用 charmbracelet/log 进行完整的操作日志
- ✅ **相对/绝对路径**：支持相对 OID 和绝对路径注册
- ✅ **运行时管理**：支持动态注册和注销 OID

## 快速开始

### 1. 创建 Agent

```go
import "github.com/liuzhen9320/snmp-go"

config := lzsnmp.Config{
    PEN:        12345,              // 你的 Private Enterprise Number
    ListenAddr: "0.0.0.0:161",      // 监听地址
    Community:  "public",            // Community string
    LogLevel:   log.InfoLevel,       // 日志级别
}

agent, err := lzsnmp.NewAgent(config)
if err != nil {
    log.Fatal(err)
}
```

### 2. 注册 OID

#### 静态值注册

```go
// 相对 OID: 1.3.6.1.4.1.12345.1.1.0
err = agent.RegisterStatic("1.1.0", gosnmp.OctetString, "My Application v1.0")

// 绝对路径 OID
err = agent.RegisterStaticAbsolute("1.3.6.1.2.1.1.1.0", 
    gosnmp.OctetString, "System Description")
```

#### 动态值注册

```go
// 系统运行时间
startTime := time.Now()
err = agent.Register("2.1.0", gosnmp.Integer, func() (interface{}, error) {
    return int(time.Since(startTime).Seconds()), nil
})

// 内存使用
err = agent.Register("2.2.0", gosnmp.Gauge32, func() (interface{}, error) {
    var m runtime.MemStats
    runtime.ReadMemStats(&m)
    return uint(m.Alloc / 1024 / 1024), nil // MB
})
```

### 3. 启动 Agent

```go
if err := agent.Start(); err != nil {
    log.Fatal(err)
}

// 优雅关闭
defer agent.Stop()
```

## API 参考

### 配置结构

```go
type Config struct {
    PEN        uint32      // Private Enterprise Number（必需）
    ListenAddr string      // 监听地址，默认 "0.0.0.0:161"
    Community  string      // Community string，默认 "public"
    LogLevel   log.Level   // 日志级别
    Logger     *log.Logger // 自定义 logger（可选）
}
```

### 主要方法

#### `Register(relativeOID, oidType, handler)`
注册相对 OID，自动添加企业前缀。

```go
agent.Register("1.1.0", gosnmp.Integer, func() (interface{}, error) {
    return 42, nil
})
// 实际 OID: 1.3.6.1.4.1.{PEN}.1.1.0
```

#### `RegisterAbsolute(oid, oidType, handler)`
注册绝对路径 OID。

```go
agent.RegisterAbsolute("1.3.6.1.2.1.1.1.0", gosnmp.OctetString, 
    func() (interface{}, error) {
        return "System Description", nil
    })
```

#### `RegisterStatic(relativeOID, oidType, value)`
注册静态值（相对路径）。

```go
agent.RegisterStatic("1.1.0", gosnmp.OctetString, "Static Value")
```

#### `RegisterStaticAbsolute(oid, oidType, value)`
注册静态值（绝对路径）。

```go
agent.RegisterStaticAbsolute("1.3.6.1.4.1.12345.1.1.0", 
    gosnmp.Integer, 100)
```

#### `Unregister(relativeOID)` / `UnregisterAbsolute(oid)`
注销 OID。

```go
agent.Unregister("1.1.0")
agent.UnregisterAbsolute("1.3.6.1.4.1.12345.1.1.0")
```

#### `GetPrefix()`
获取企业 OID 前缀。

```go
prefix := agent.GetPrefix() // "1.3.6.1.4.1.12345"
```

#### `ListOIDs()`
列出所有已注册的 OID。

```go
oids := agent.ListOIDs()
for oid, oidType := range oids {
    fmt.Printf("%s: %s\n", oid, oidType) // "dynamic" 或 "static"
}
```

## 支持的数据类型

使用 `gosnmp.Asn1BER` 类型：

- `gosnmp.Integer` - 整数
- `gosnmp.OctetString` - 字符串
- `gosnmp.Counter32` - 32 位计数器
- `gosnmp.Counter64` - 64 位计数器
- `gosnmp.Gauge32` - 32 位仪表
- `gosnmp.TimeTicks` - 时间刻度
- `gosnmp.IPAddress` - IP 地址

## 使用示例

### 监控应用指标

```go
// 请求计数器
var requestCount uint64
agent.Register("3.1.0", gosnmp.Counter64, func() (interface{}, error) {
    return atomic.LoadUint64(&requestCount), nil
})

// 错误率
agent.Register("3.2.0", gosnmp.Gauge32, func() (interface{}, error) {
    errorRate := calculateErrorRate()
    return uint(errorRate * 100), nil // 百分比
})

// 活跃连接数
agent.Register("3.3.0", gosnmp.Integer, func() (interface{}, error) {
    return getActiveConnections(), nil
})
```

### 系统信息

```go
// CPU 使用率
agent.Register("4.1.0", gosnmp.Gauge32, func() (interface{}, error) {
    cpuPercent := getCPUUsage()
    return uint(cpuPercent), nil
})

// 磁盘空间
agent.Register("4.2.0", gosnmp.Gauge32, func() (interface{}, error) {
    diskFree := getDiskFreeSpace() // GB
    return uint(diskFree), nil
})
```

## 测试

使用 `snmpget` 和 `snmpwalk` 工具测试：

```bash
# 获取单个 OID
snmpget -v2c -c public localhost:161 1.3.6.1.4.1.12345.1.1.0

# 遍历整个企业树
snmpwalk -v2c -c public localhost:161 1.3.6.1.4.1.12345

# 使用 snmptable（适用于表格数据）
snmptable -v2c -c public localhost:161 1.3.6.1.4.1.12345.5
```

## 日志示例

```
2025-10-31T10:30:15+08:00 INFO SNMP Agent initialized pen=12345 prefix=1.3.6.1.4.1.12345
2025-10-31T10:30:15+08:00 INFO Registered static OID oid=1.3.6.1.4.1.12345.1.1.0 type=OctetString
2025-10-31T10:30:15+08:00 INFO Registered dynamic OID oid=1.3.6.1.4.1.12345.2.1.0 type=Integer
2025-10-31T10:30:15+08:00 INFO Starting SNMP Agent addr=0.0.0.0:161
2025-10-31T10:30:20+08:00 DEBUG GET request oid=1.3.6.1.4.1.12345.2.1.0
2025-10-31T10:30:20+08:00 DEBUG GET response oid=1.3.6.1.4.1.12345.2.1.0 value=125
```