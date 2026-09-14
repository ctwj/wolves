package vo

import (
	"database/sql/driver"
	"encoding/json"
)

// Extends 额外扩展值对象,用在文章详情扩展
type Extends []ExtendsItem

func (ext Extends) Get(key string) any {
	for _, item := range ext {
		if item.Key == key {
			return item.Value
		}
	}
	return nil
}

// Set 写入键值：已存在则覆盖，否则追加（采集插件记录 source_url 等溯源信息用）
func (ext *Extends) Set(key string, value any) {
	for i, item := range *ext {
		if item.Key == key {
			(*ext)[i].Value = value
			return
		}
	}
	*ext = append(*ext, ExtendsItem{Key: key, Value: value})
}

func (ext *Extends) Scan(value interface{}) error {
	s, _ := value.(string)
	if len(s) == 0 {
		*ext = Extends{}
		return nil
	}
	_ = json.Unmarshal([]byte(s), ext)
	return nil
}

func (ext Extends) Value() (driver.Value, error) {
	b, err := json.Marshal(&ext)
	if err != nil || len(b) == 0 {
		return "", err
	}
	return string(b), nil
}

type ExtendsItem struct {
	Key   string           `json:"key"`
	Value ExtendsItemValue `json:"value"`
}

type ExtendsItemValue any
