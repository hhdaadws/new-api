package ratio_setting

import (
	"github.com/QuantumNous/new-api/types"
)

var imageSizePriceMap = types.NewRWMap[string, map[string]float64]()

func GetImageSizePrice(model, size string) (float64, bool) {
	model = FormatMatchingModelName(model)
	sizeMap, ok := imageSizePriceMap.Get(model)
	if !ok {
		return 0, false
	}
	price, ok := sizeMap[size]
	return price, ok
}

func HasImageSizePrice(model string) bool {
	model = FormatMatchingModelName(model)
	sizeMap, ok := imageSizePriceMap.Get(model)
	return ok && len(sizeMap) > 0
}

func GetImageSizePriceForModel(model string) (map[string]float64, bool) {
	model = FormatMatchingModelName(model)
	return imageSizePriceMap.Get(model)
}

func GetImageSizePriceCopy() map[string]map[string]float64 {
	return imageSizePriceMap.ReadAll()
}

func ImageSizePrice2JSONString() string {
	return imageSizePriceMap.MarshalJSONString()
}

func UpdateImageSizePriceByJSONString(jsonStr string) error {
	return types.LoadFromJsonStringWithCallback(imageSizePriceMap, jsonStr, InvalidateExposedDataCache)
}
