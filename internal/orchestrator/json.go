package orchestrator

import "encoding/json"

// jsonMarshalWithErrors is a tiny indirection used by HostResult's
// custom MarshalJSON. Lifted out so the helper signature stays clean
// and so future custom marshallers can reuse the pattern.
func jsonMarshalWithErrors(v any) ([]byte, error) {
	return json.Marshal(v)
}
