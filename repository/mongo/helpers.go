package mongo

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func toString(v interface{}) string {
	if v == nil {
		return ""
	}
	switch s := v.(type) {
	case string:
		return s
	case primitive.ObjectID:
		return s.Hex()
	default:
		return ""
	}
}

func toBool(v interface{}) bool {
	if v == nil {
		return false
	}
	switch b := v.(type) {
	case bool:
		return b
	default:
		return false
	}
}

func toFloat64(v interface{}) float64 {
	if v == nil {
		return 0
	}
	switch i := v.(type) {
	case float64:
		return i
	case int32:
		return float64(i)
	case int:
		return float64(i)
	case int64:
		return float64(i)
	default:
		return 0
	}
}

func objectIDToString(v interface{}) string {
	switch id := v.(type) {
	case string:
		return id
	case primitive.ObjectID:
		return id.Hex()
	default:
		return ""
	}
}

func toInt(v interface{}) int {
	if v == nil {
		return 0
	}
	switch i := v.(type) {
	case int:
		return i
	case int32:
		return int(i)
	case int64:
		return int(i)
	case float64:
		return int(i)
	default:
		return 0
	}
}
