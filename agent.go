package lzsnmp

import (
	"fmt"
	"os"
	"sync"

	"github.com/charmbracelet/log"
	"github.com/gosnmp/gosnmp"
	"github.com/slayercat/GoSNMPServer"
)

// ValueHandler 动态值处理函数类型
type ValueHandler func() (interface{}, error)

// Config SNMP Agent 配置
type Config struct {
	PEN        uint32 // Private Enterprise Number
	ListenAddr string // 监听地址，如 "0.0.0.0:161"
	Community  string // Community string，默认 "public"
	LogLevel   log.Level
	Logger     *log.Logger
}

// Agent SNMP Agent 封装
type Agent struct {
	config     Config
	server     *GoSNMPServer.MasterAgent
	snmpServer *GoSNMPServer.SNMPServer
	logger     *log.Logger
	oidPrefix  string
	handlers   map[string]ValueHandler
	staticVals map[string]interface{}
	mu         sync.RWMutex
}

// OIDEntry OID 注册项
type OIDEntry struct {
	OID     string
	Type    gosnmp.Asn1BER
	Handler ValueHandler
	Static  interface{}
}

// NewAgent 创建新的 SNMP Agent
func NewAgent(cfg Config) (*Agent, error) {
	if cfg.PEN == 0 {
		return nil, fmt.Errorf("PEN (Private Enterprise Number) is required")
	}

	if cfg.ListenAddr == "" {
		cfg.ListenAddr = "0.0.0.0:161"
	}

	if cfg.Community == "" {
		cfg.Community = "public"
	}

	// 初始化日志
	logger := cfg.Logger
	if logger == nil {
		logger = log.NewWithOptions(os.Stderr, log.Options{
			ReportTimestamp: true,
			ReportCaller:    cfg.LogLevel == log.DebugLevel,
			Level:           cfg.LogLevel,
			Prefix:          "lzsnmp",
		})
	}

	// 生成企业 OID 前缀
	oidPrefix := fmt.Sprintf("1.3.6.1.4.1.%d", cfg.PEN)

	agent := &Agent{
		config:     cfg,
		logger:     logger,
		oidPrefix:  oidPrefix,
		handlers:   make(map[string]ValueHandler),
		staticVals: make(map[string]interface{}),
	}

	logger.Info("SNMP Agent initialized",
		"pen", cfg.PEN,
		"prefix", oidPrefix,
		"listen", cfg.ListenAddr)

	return agent, nil
}

// Start 启动 SNMP Agent
func (a *Agent) Start() error {
	a.logger.Info("Starting SNMP Agent", "addr", a.config.ListenAddr)

	master := GoSNMPServer.MasterAgent{
		SecurityConfig: GoSNMPServer.SecurityConfig{
			AuthoritativeEngineBoots: 1,
			Users:                    []gosnmp.UsmSecurityParameters{},
		},
		SubAgents: []*GoSNMPServer.SubAgent{
			{
				CommunityIDs: []string{a.config.Community},
				OIDs:         []*GoSNMPServer.PDUValueControlItem{},
			},
		},
	}

	a.server = &master
	a.snmpServer = GoSNMPServer.NewSNMPServer(master)

	// 注册处理器
	a.registerHandlers()

	// 启动服务器
	if err := a.snmpServer.ListenUDP("udp", a.config.ListenAddr); err != nil {
		a.logger.Error("Failed to start SNMP server", "error", err)
		return fmt.Errorf("failed to start SNMP server: %w", err)
	}

	// 启动服务循环
	go func() {
		a.logger.Debug("Starting SNMP server loop")
		a.snmpServer.ServeForever()
	}()

	a.logger.Info("SNMP Agent started successfully")
	return nil
}

// Stop 停止 SNMP Agent
func (a *Agent) Stop() error {
	a.logger.Info("Stopping SNMP Agent")
	if a.snmpServer != nil {
		a.snmpServer.Shutdown()
	}
	return nil
}

// Register 注册相对 OID
func (a *Agent) Register(relativeOID string, oidType gosnmp.Asn1BER, handler ValueHandler) error {
	absoluteOID := fmt.Sprintf("%s.%s", a.oidPrefix, relativeOID)
	return a.RegisterAbsolute(absoluteOID, oidType, handler)
}

// RegisterAbsolute 注册绝对路径 OID
func (a *Agent) RegisterAbsolute(oid string, oidType gosnmp.Asn1BER, handler ValueHandler) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if _, exists := a.handlers[oid]; exists {
		a.logger.Warn("OID already registered, overwriting", "oid", oid)
	}

	a.handlers[oid] = handler
	a.logger.Info("Registered dynamic OID", "oid", oid, "type", oidType)

	// 如果服务器已启动，更新处理器
	if a.server != nil {
		a.registerHandlers()
	}

	return nil
}

// RegisterStatic 注册静态值
func (a *Agent) RegisterStatic(relativeOID string, oidType gosnmp.Asn1BER, value interface{}) error {
	absoluteOID := fmt.Sprintf("%s.%s", a.oidPrefix, relativeOID)
	return a.RegisterStaticAbsolute(absoluteOID, oidType, value)
}

// RegisterStaticAbsolute 注册绝对路径静态值
func (a *Agent) RegisterStaticAbsolute(oid string, oidType gosnmp.Asn1BER, value interface{}) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if _, exists := a.staticVals[oid]; exists {
		a.logger.Warn("Static OID already registered, overwriting", "oid", oid)
	}

	a.staticVals[oid] = value
	a.logger.Info("Registered static OID", "oid", oid, "type", oidType, "value", value)

	// 如果服务器已启动，更新处理器
	if a.server != nil {
		a.registerHandlers()
	}

	return nil
}

// Unregister 注销 OID
func (a *Agent) Unregister(relativeOID string) error {
	absoluteOID := fmt.Sprintf("%s.%s", a.oidPrefix, relativeOID)
	return a.UnregisterAbsolute(absoluteOID)
}

// UnregisterAbsolute 注销绝对路径 OID
func (a *Agent) UnregisterAbsolute(oid string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	deletedHandler := false
	deletedStatic := false

	if _, exists := a.handlers[oid]; exists {
		delete(a.handlers, oid)
		deletedHandler = true
	}

	if _, exists := a.staticVals[oid]; exists {
		delete(a.staticVals, oid)
		deletedStatic = true
	}

	if !deletedHandler && !deletedStatic {
		a.logger.Warn("OID not found for unregistration", "oid", oid)
		return fmt.Errorf("OID not found: %s", oid)
	}

	a.logger.Info("Unregistered OID", "oid", oid)

	// 如果服务器已启动，更新处理器
	if a.server != nil {
		a.registerHandlers()
	}

	return nil
}

// registerHandlers 将所有注册的 OID 注册到 SNMP 服务器
func (a *Agent) registerHandlers() {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if a.server == nil || len(a.server.SubAgents) == 0 {
		return
	}

	subAgent := a.server.SubAgents[0]
	subAgent.OIDs = []*GoSNMPServer.PDUValueControlItem{}

	// 注册动态处理器
	for oid, handler := range a.handlers {
		oidCopy := oid
		handlerCopy := handler

		pduItem := &GoSNMPServer.PDUValueControlItem{
			OID:  oidCopy,
			Type: gosnmp.OctetString,
			OnGet: func() (interface{}, error) {
				a.logger.Debug("GET request", "oid", oidCopy)
				value, err := handlerCopy()
				if err != nil {
					a.logger.Error("Handler error", "oid", oidCopy, "error", err)
					return nil, err
				}
				a.logger.Debug("GET response", "oid", oidCopy, "value", value)
				return value, nil
			},
		}

		subAgent.OIDs = append(subAgent.OIDs, pduItem)
	}

	// 注册静态值
	for oid, value := range a.staticVals {
		oidCopy := oid
		valueCopy := value

		pduItem := &GoSNMPServer.PDUValueControlItem{
			OID:  oidCopy,
			Type: gosnmp.OctetString, // 使用类型转换
			OnGet: func() (interface{}, error) {
				a.logger.Debug("GET request (static)", "oid", oidCopy, "value", valueCopy)
				return valueCopy, nil
			},
		}

		subAgent.OIDs = append(subAgent.OIDs, pduItem)
	}

	a.logger.Debug("Handlers registered",
		"dynamic", len(a.handlers),
		"static", len(a.staticVals))
}

// GetPrefix 获取企业 OID 前缀
func (a *Agent) GetPrefix() string {
	return a.oidPrefix
}

// ListOIDs 列出所有已注册的 OID
func (a *Agent) ListOIDs() map[string]string {
	a.mu.RLock()
	defer a.mu.RUnlock()

	result := make(map[string]string)

	for oid := range a.handlers {
		result[oid] = "dynamic"
	}

	for oid := range a.staticVals {
		result[oid] = "static"
	}

	return result
}
