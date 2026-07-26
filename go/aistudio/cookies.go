package aistudio

import (
	"net/http"
	"strings"
)

type CookieRecord struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type CookieJar struct {
	values map[string]string
	order  []string
}

func NewCookieJar(header string) *CookieJar {
	jar := &CookieJar{values: map[string]string{}}
	jar.SetHeader(header)
	return jar
}
func (j *CookieJar) SetHeader(header string) {
	j.values, j.order = map[string]string{}, nil
	for _, pair := range strings.Split(header, ";") {
		fields := strings.SplitN(strings.TrimSpace(pair), "=", 2)
		if len(fields) == 2 && fields[0] != "" {
			j.values[fields[0]] = fields[1]
			j.order = append(j.order, fields[0])
		}
	}
}
func (j *CookieJar) Header() string {
	pairs := make([]string, 0, len(j.order))
	for _, name := range j.order {
		if value, ok := j.values[name]; ok {
			pairs = append(pairs, name+"="+value)
		}
	}
	return strings.Join(pairs, "; ")
}
func (j *CookieJar) Apply(records []CookieRecord) {
	for _, record := range records {
		if record.Name == "" {
			continue
		}
		if _, ok := j.values[record.Name]; !ok {
			j.order = append(j.order, record.Name)
		}
		j.values[record.Name] = record.Value
	}
}
func (j *CookieJar) ApplyResponse(response *http.Response) {
	for _, cookie := range response.Cookies() {
		j.Apply([]CookieRecord{{Name: cookie.Name, Value: cookie.Value}})
	}
}
