package reference

const (
	// MaxFileBytes is the maximum source-file size accepted by the reader.
	MaxFileBytes = 16 << 20
	// MaxExtractedText is the maximum decoded text size accepted by the reader.
	MaxExtractedText = 4 << 20
	// MaxPDFPages is the maximum PDF page count accepted by the reader.
	MaxPDFPages              = 500
	maxCatalogBytes          = 4096
	maxSelectedFiles         = 16
	maxReferenceBytes        = 32768
	maxSelectionOutputTokens = 1024
	defaultContextTokens     = 8192
)
