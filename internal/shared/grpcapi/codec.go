package grpcapi

import (
	"encoding/json"

	"google.golang.org/grpc/encoding"
)

const JSONCodecName = "json"

type jsonCodec struct{}

func (jsonCodec) Name() string {
	return JSONCodecName
}

func (jsonCodec) Marshal(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}

func (jsonCodec) Unmarshal(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}

func init() {
	encoding.RegisterCodec(jsonCodec{})
}
