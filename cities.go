package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

const cityListURL = "https://j.i8tq.com/weather2020/search/city.js"

type cityEntry struct {
	AreaID string `json:"AREAID"`
	Name   string `json:"NAMECN"`
}
type cityData map[string]map[string]map[string]cityEntry

type cityResolver struct {
	mu      sync.Mutex
	client  *http.Client
	url     string
	data    cityData
	updated time.Time
}

var weatherCities = &cityResolver{client: weatherHTTPClient, url: cityListURL}

// resolve 根据中国天气网城市列表查找代码；列表不可用时使用已有的常用城市备份。
func (r *cityResolver) resolve(name string) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.data) == 0 || time.Since(r.updated) > 24*time.Hour {
		data, err := r.fetch()
		if err == nil {
			r.data = data
			r.updated = time.Now()
		} else if len(r.data) == 0 {
			if id := weatherCityIDs[strings.TrimSuffix(strings.TrimSpace(name), "市")]; cityIDPattern.MatchString(id) {
				return id, nil
			}
			return "", fmt.Errorf("无法获取城市列表: %w", err)
		}
	}
	return lookupCityID(r.data, name)
}
func (r *cityResolver) fetch() (cityData, error) {
	req, err := http.NewRequest(http.MethodGet, r.url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; KindleDashboard/1.0)")
	req.Header.Set("Referer", "https://www.weather.com.cn/")
	resp, err := r.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("城市列表 HTTP %s", resp.Status)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return nil, err
	}
	start := strings.IndexByte(string(b), '{')
	if start < 0 {
		return nil, fmt.Errorf("城市列表格式错误")
	}
	raw := strings.TrimSuffix(strings.TrimSpace(string(b[start:])), ";")
	var data cityData
	if err = json.Unmarshal([]byte(raw), &data); err != nil || len(data) == 0 {
		return nil, fmt.Errorf("城市列表解析失败")
	}
	return data, nil
}

// lookupCityID 优先匹配城市本身；同名地区可使用“省/城市”指定。
func lookupCityID(data cityData, input string) (string, error) {
	input = strings.TrimSpace(input)
	parts := strings.Split(input, "/")
	if len(parts) > 2 || input == "" {
		return "", fmt.Errorf("无效城市名 %q", input)
	}
	name := strings.TrimSuffix(strings.TrimSpace(parts[len(parts)-1]), "市")
	province := ""
	if len(parts) == 2 {
		province = strings.TrimSuffix(strings.TrimSpace(parts[0]), "省")
		if province == "" || name == "" {
			return "", fmt.Errorf("无效城市名 %q", input)
		}
	}
	type result struct{ label, id string }
	var cities, areas []result
	for prov, groups := range data {
		if province != "" && strings.TrimSuffix(prov, "省") != province {
			continue
		}
		for city, entries := range groups {
			if strings.TrimSuffix(city, "市") == name {
				if entry, ok := entries[city]; ok && cityIDPattern.MatchString(entry.AreaID) {
					cities = append(cities, result{prov + "/" + city, entry.AreaID})
				}
			}
			for area, entry := range entries {
				if strings.TrimSuffix(area, "市") == name && cityIDPattern.MatchString(entry.AreaID) {
					areas = append(areas, result{prov + "/" + city + "/" + area, entry.AreaID})
				}
			}
		}
	}
	candidates := cities
	if len(candidates) == 0 {
		candidates = areas
	}
	unique := map[string]string{}
	for _, item := range candidates {
		unique[item.id] = item.label
	}
	if len(unique) == 1 {
		for id := range unique {
			return id, nil
		}
	}
	if len(unique) > 1 {
		labels := make([]string, 0, len(unique))
		for _, label := range unique {
			labels = append(labels, label)
		}
		sort.Strings(labels)
		return "", fmt.Errorf("城市名 %q 有多个匹配，请填写省/城市：%s", input, strings.Join(labels, "、"))
	}
	return "", fmt.Errorf("未找到城市 %q，请检查城市名称", input)
}
