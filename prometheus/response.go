package prometheus

import (
	"encoding/json"
	"fmt"
	"strconv"
)

type ValuesResponse struct {
	Status string   `json:"status"`
	Data   []string `json:"data"`
}

type MatrixResponse struct {
	Status string       `json:"status"`
	Data   MatrixResult `json:"data"`
}

type MatrixResult struct {
	Result     []MatrixData `json:"result"`
	ResultType string       `json:"resultType"`
}

type MatrixData struct {
	Metric map[string]string `json:"metric"`
	Values []MatrixPair      `json:"values"`
}

type MatrixPair struct {
	Timestamp float64
	Value     float64
}

func (m *MatrixPair) UnmarshalJSON(data []byte) error {
	arr := make([]interface{}, 0)
	err := json.Unmarshal(data, &arr)
	if err != nil {
		return err
	}

	if len(arr) != 2 {
		return fmt.Errorf("length mismatch, got %v, expected 2", len(arr))
	}

	timestamp, ok := arr[0].(float64)
	if !ok {
		return fmt.Errorf("type mismatch for element[0/1], expected 'float64', got '%T', str=%v", arr[0], string(data))
	}
	m.Timestamp = timestamp

	str, ok := arr[1].(string)
	if !ok {
		return fmt.Errorf("type mismatch for element[1/1], expected 'string', got '%T', str=%v", arr[1], string(data))
	}

	f, err := strconv.ParseFloat(str, 64)
	if err != nil {
		return err
	}
	m.Value = f
	return nil
}
