// +build linux,!ppc64le

package cntk

// #include <stdlib.h>
// #include "cbits/predict.hpp"
import "C"
import (
	"encoding/json"
	"unsafe"

	"github.com/Unknwon/com"
	"github.com/pkg/errors"
	"github.com/rai-project/dlframework/framework/options"
	nvidiasmi "github.com/rai-project/nvidia-smi"
)

type Predictor struct {
	ctx     C.PredictorContext
	options *options.Options
}

func New(opts0 ...options.Option) (*Predictor, error) {
	opts := options.New(opts0...)
	modelFile := string(opts.Graph())
	if !com.IsFile(modelFile) {
		return nil, errors.Errorf("file %s not found", modelFile)
	}
	modelFileString := C.CString(modelFile)
	defer C.free(unsafe.Pointer(modelFileString))
	deviceType := "CPU"
	deviceId := 0
	if opts.UsesGPU() {
		if !nvidiasmi.HasGPU {
			return nil, errors.New("no GPU device")
		}
		deviceType = "GPU"
		for _, d := range opts.Devices() {
			if d.Type() == options.CUDA_DEVICE {
				deviceId = d.ID()
				break
			}
		}
	}

	deviceTypeString := C.CString(deviceType)
	defer C.free(unsafe.Pointer(deviceTypeString))

	ctx := C.NewCNTK(
		modelFileString,
		deviceTypeString,
		C.int(deviceId),
	)
	return &Predictor{
		ctx:     ctx,
		options: opts,
	}, nil
}

func prod(arry []uint32) int64 {
	accum := int64(1)
	for _, e := range arry {
		accum *= int64(e)
	}
	return accum
}

func (p *Predictor) Predict(input []float32, outputLayerName0 string, shape []uint32) (Predictions, error) {
	if outputLayerName0 == "" {
		return nil, errors.New("expecting a valid (non-empty) output layer name")
	}
	outputLayerName := C.CString(outputLayerName0)
	defer C.free(unsafe.Pointer(outputLayerName))

	batchSize := int64(p.options.BatchSize())
	shapeLen := prod(shape)
	dataLen := int64(len(input)) / shapeLen
	if batchSize > dataLen {
		padding := make([]float32, (batchSize-dataLen)*shapeLen)
		input = append(input, padding...)
	}

	ptr := (*C.float)(unsafe.Pointer(&input[0]))
	r := C.PredictCNTK(p.ctx, ptr, outputLayerName, C.int(batchSize))
	if r == nil {
		return nil, errors.New("failed to perform CNTK prediction")
	}
	defer C.free(unsafe.Pointer(r))
	js := C.GoString(r)

	predictions := []Prediction{}
	err := json.Unmarshal([]byte(js), &predictions)
	if err != nil {
		return nil, err
	}
	return predictions, nil
}
func (p *Predictor) StartProfiling(name, metadata string) error {
	cname := C.CString(name)
	cmetadata := C.CString(metadata)
	defer C.free(unsafe.Pointer(cname))
	defer C.free(unsafe.Pointer(cmetadata))
	C.CNTKStartProfiling(p.ctx, cname, cmetadata)
	return nil
}

func (p *Predictor) EndProfiling() error {
	C.CNTKEndProfiling(p.ctx)
	return nil
}

func (p *Predictor) DisableProfiling() error {
	C.CNTKDisableProfiling(p.ctx)
	return nil
}

func (p *Predictor) ReadProfile() (string, error) {
	cstr := C.CNTKReadProfile(p.ctx)
	if cstr == nil {
		return "", errors.New("failed to read nil profile")
	}
	defer C.free(unsafe.Pointer(cstr))
	return C.GoString(cstr), nil
}

func (p Predictor) Close() {
	C.DeleteCNTK(p.ctx)
}
