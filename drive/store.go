package drive

import (
	"errors"
)

type DriveIndex struct {
	drives map[string]UserDrive
}

type DriveStore interface {
	Resolve(string) (UserDrive, error)
	ResolveCreate(string) (UserDrive, error)
	Put(UserDrive) error
	Update(UserDrive) error
}

func NewDriveStore() *DriveIndex {
	return &DriveIndex{drives: make(map[string]UserDrive)}
}

func (di *DriveIndex) Resolve(userDID string) (UserDrive, error) {
	if d, ok := di.drives[userDID]; ok {
		return d, nil
	}

	return UserDrive{}, errors.New("Drive not found")
}

func (di *DriveIndex) ResolveCreate(userDID string) (UserDrive, error) {
	d, err := di.Resolve(userDID)
	if err == nil {
		return d, nil
	}

	d = NewDrive(userDID, di)
	di.Put(d)
	return d, nil
}

func (di *DriveIndex) Put(d UserDrive) error {
	if _, ok := di.drives[d.UserDID]; ok {
		return errors.New("drive already exists")
	}

	di.drives[d.UserDID] = d
	return nil
}

func (di *DriveIndex) Update(d UserDrive) error {
	if _, ok := di.drives[d.UserDID]; ok {
		di.drives[d.UserDID] = d
		return nil
	}

	return errors.New("drive does not exist")
}
