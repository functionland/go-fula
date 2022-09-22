package drive

import (
	"errors"
)

// DriveIndex an in-memory list of drives which acts as a testing infra for DriveStore
type DriveIndex struct {
	drives map[string]UserDrive
}

// DriveStore Interface for a Drive Store. DriveStore handles logic for discovering, updating and persisting a drive
type DriveStore interface {
	Resolve(string) (UserDrive, error)
	ResolveCreate(string) (UserDrive, error)
	Put(UserDrive) error
	Update(UserDrive) error
}

// NewDriveStore create a DriveIndex with an empty list of drives
func NewDriveStore() *DriveIndex {
	return &DriveIndex{drives: make(map[string]UserDrive)}
}

// Resolve a Drive based on UserDID, if the drive does not exist returns a "Drive not found" error
func (di *DriveIndex) Resolve(userDID string) (UserDrive, error) {
	if d, ok := di.drives[userDID]; ok {
		return d, nil
	}

	return UserDrive{}, errors.New("Drive not found")
}

// ResolveCreate resolve a Drive based on UserDID, if the drive does not exist creates one, puts it in the store and returns it
func (di *DriveIndex) ResolveCreate(userDID string) (UserDrive, error) {
	d, err := di.Resolve(userDID)
	if err == nil {
		return d, nil
	}

	d = NewDrive(userDID, di)
	di.Put(d)
	return d, nil
}

// Put a new drive in the DriveStore, if the drive already exists, returns a "drive already exists" error
func (di *DriveIndex) Put(d UserDrive) error {
	if _, ok := di.drives[d.UserDID]; ok {
		return errors.New("drive already exists")
	}

	di.drives[d.UserDID] = d
	return nil
}

// Update an existing drive in the Drive store, if the drive does not exist, returns a "drive does not exist" error
func (di *DriveIndex) Update(d UserDrive) error {
	if _, ok := di.drives[d.UserDID]; ok {
		di.drives[d.UserDID] = d
		return nil
	}

	return errors.New("drive does not exist")
}
