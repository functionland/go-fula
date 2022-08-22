package drive

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
)

const DRIVES_JSON_PATH = "./drives.json"

func LoadDrivesJSON() (*Drives, error) {
	drivesFile, err := os.Open(DRIVES_JSON_PATH)
	if err != nil {
		fmt.Println("could not open drives.json", err)
		return nil, err
	}

	cdBytes, err := ioutil.ReadAll(drivesFile)
	if err != nil {
		fmt.Println("error reading drives.json", err)
		return nil, err
	}

	var drives Drives
	json.Unmarshal(cdBytes, &drives)

	return &drives, nil
}

func WriteDrivesJSON(drives Drives) error {
	ndFile, err := json.MarshalIndent(drives, "", " ")
	if err != nil {
		fmt.Println("error in writing new drives", err)
		return err
	}

	err = ioutil.WriteFile(DRIVES_JSON_PATH, ndFile, 0644)
	if err != nil {
		fmt.Println("error in writing new drives", err)
		return err
	}

	return nil
}

func ResolveDrive(userDID string) (*UserDrive, error) {
	drives, err := LoadDrivesJSON()
	if err != nil {
		fmt.Println("error in loading drives", err)
		return nil, err
	}

	for _, ud := range drives.Drives {
		if ud.UserDID == userDID {
			return &ud, nil
		}
	}

	return NewDrive(userDID), errors.New("drive not found")
}

func PutDrive(ud UserDrive) error {
	drives, err := LoadDrivesJSON()
	if err != nil {
		fmt.Println("error in loading drives", err)
		return err
	}

	found := false
	for idx, dr := range drives.Drives {
		if dr.UserDID == ud.UserDID {
			drives.Drives[idx].PrivateSpaceCid = ud.PrivateSpaceCid
			drives.Drives[idx].PublicSpaceCid = ud.PublicSpaceCid
			found = true
		}
	}

	if !found {
		drives.Drives = append(drives.Drives, ud)
	}

	// err = WriteDrivesJSON(*drives)
	// if err != nil {
	// 	fmt.Println("error in writing drivrd to json", err)
	// 	return err
	// }

	return nil
}
