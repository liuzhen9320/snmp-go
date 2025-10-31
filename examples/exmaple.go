package main

import (
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/charmbracelet/log"
	"github.com/gosnmp/gosnmp"
	lzsnmp "github.com/liuzhen9320/snmp-go"
)

func main() {
	// 创建 SNMP Agent 配置
	config := lzsnmp.Config{
		PEN:        12345,          // 你的 Private Enterprise Number
		ListenAddr: "0.0.0.0:1161", // 使用 1161 端口避免需要 root 权限
		Community:  "public",
		LogLevel:   log.DebugLevel,
	}

	// 创建 Agent
	agent, err := lzsnmp.NewAgent(config)
	if err != nil {
		log.Fatal("Failed to create agent", "error", err)
	}

	// 1. 注册静态值
	// OID: 1.3.6.1.4.1.12345.1.1.0
	err = agent.RegisterStatic("1.1.0", gosnmp.OctetString, "My SNMP Agent v1.0")
	if err != nil {
		log.Error("Failed to register static OID", "error", err)
	}

	// OID: 1.3.6.1.4.1.12345.1.2.0
	err = agent.RegisterStatic("1.2.0", gosnmp.OctetString, "Example Device")
	if err != nil {
		log.Error("Failed to register static OID", "error", err)
	}

	// 2. 注册动态值 - 系统运行时间
	// OID: 1.3.6.1.4.1.12345.2.1.0
	startTime := time.Now()
	err = agent.Register("2.1.0", gosnmp.Integer, func() (interface{}, error) {
		uptime := int(time.Since(startTime).Seconds())
		return uptime, nil
	})
	if err != nil {
		log.Error("Failed to register uptime OID", "error", err)
	}

	// 3. 注册动态值 - 内存使用
	// OID: 1.3.6.1.4.1.12345.2.2.0
	err = agent.Register("2.2.0", gosnmp.Gauge32, func() (interface{}, error) {
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		// 返回已分配的内存（MB）
		return uint(m.Alloc / 1024 / 1024), nil
	})
	if err != nil {
		log.Error("Failed to register memory OID", "error", err)
	}

	// 4. 注册动态值 - Goroutine 数量
	// OID: 1.3.6.1.4.1.12345.2.3.0
	err = agent.Register("2.3.0", gosnmp.Integer, func() (interface{}, error) {
		return runtime.NumGoroutine(), nil
	})
	if err != nil {
		log.Error("Failed to register goroutine OID", "error", err)
	}

	// 5. 注册动态值 - 当前时间戳
	// OID: 1.3.6.1.4.1.12345.2.4.0
	err = agent.Register("2.4.0", gosnmp.Integer, func() (interface{}, error) {
		return int(time.Now().Unix()), nil
	})
	if err != nil {
		log.Error("Failed to register timestamp OID", "error", err)
	}

	// 6. 注册绝对路径 OID（标准 OID）
	// sysDescr: 1.3.6.1.2.1.1.1.0
	err = agent.RegisterStaticAbsolute("1.3.6.1.2.1.1.1.0", gosnmp.OctetString,
		"Custom SNMP Agent on Linux")
	if err != nil {
		log.Error("Failed to register sysDescr", "error", err)
	}

	// 7. 计数器示例
	// OID: 1.3.6.1.4.1.12345.3.1.0
	counter := 0
	err = agent.Register("3.1.0", gosnmp.Counter32, func() (interface{}, error) {
		counter++
		return uint(counter), nil
	})
	if err != nil {
		log.Error("Failed to register counter OID", "error", err)
	}

	// 列出所有注册的 OID
	log.Info("Registered OIDs:")
	for oid, oidType := range agent.ListOIDs() {
		log.Info("  -", "oid", oid, "type", oidType)
	}

	// 启动 Agent
	if err := agent.Start(); err != nil {
		log.Fatal("Failed to start agent", "error", err)
	}

	log.Info("SNMP Agent is running")
	log.Info("Test with: snmpget -v2c -c public 127.0.0.1:1161 " + agent.GetPrefix() + ".1.1.0")
	log.Info("Or: snmpwalk -v2c -c public 127.0.0.1:1161 " + agent.GetPrefix())

	// 等待中断信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	log.Info("Shutting down...")

	go func() {
		agent.Stop()
	}()

	time.Sleep(5 * time.Second)

	os.Exit(0)
}
