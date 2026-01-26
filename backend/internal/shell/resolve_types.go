package shell

type BinarySource string

const (
	BinarySourceBundled  BinarySource = "bundled"
	BinarySourceSystem   BinarySource = "system"
	BinarySourceExplicit BinarySource = "explicit"
)

type ResolvedBinary struct {
	Path   string
	Source BinarySource
}
