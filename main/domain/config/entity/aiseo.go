package entity

type AISEO struct {
	RSSEnable  bool `json:"rss_enable"`
	LLMsEnable bool `json:"llms_enable"`
	APIEnable  bool `json:"api_enable"`
	RSSLimit   int  `json:"rss_limit"`
	APILimit   int  `json:"api_limit"`
	LLMsLimit  int  `json:"llms_limit"`
}

func NewAISEO() *AISEO {
	return &AISEO{
		RSSEnable:  false,
		LLMsEnable: false,
		APIEnable:  false,
		RSSLimit:   50,
		APILimit:   20,
		LLMsLimit:  20,
	}
}

func (*AISEO) ConfigID() string {
	return "aiseo"
}
