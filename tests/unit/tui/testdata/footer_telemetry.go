package testdata

type FooterTelemetryFixture struct {
	TokenStatus      string
	CacheStatus      string
	ContextUsed      int
	ContextMax       int
	ContextPercent   int
	UnknownCapacity  bool
}

var KnownCapacityFooter = FooterTelemetryFixture{
	TokenStatus:    "↑20 ↓3 R3 W6 $0.18",
	CacheStatus:    "ctx:l1-hit",
	ContextUsed:    12000,
	ContextMax:     128000,
	ContextPercent: 9,
}

var UnknownCapacityFooter = FooterTelemetryFixture{
	TokenStatus:     "↑20 ↓3 R3 W6 $0.18",
	CacheStatus:     "ctx:rebuild",
	UnknownCapacity: true,
}
