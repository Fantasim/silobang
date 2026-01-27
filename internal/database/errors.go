package database

import "fmt"

// DuplicateAssetError is returned when attempting to insert an asset that already exists
type DuplicateAssetError struct {
	Hash          string
	ExistingTopic string
}

func (e *DuplicateAssetError) Error() string {
	return fmt.Sprintf("asset %s already exists in topic %s", e.Hash, e.ExistingTopic)
}
