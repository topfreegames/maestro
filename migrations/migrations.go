// Code generated by go-bindata.
// sources:
// migrations/0001-CreateSchedulerTable.sql
// DO NOT EDIT!

package migrations

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func bindataRead(data []byte, name string) ([]byte, error) {
	gz, err := gzip.NewReader(bytes.NewBuffer(data))
	if err != nil {
		return nil, fmt.Errorf("Read %q: %v", name, err)
	}

	var buf bytes.Buffer
	_, err = io.Copy(&buf, gz)
	clErr := gz.Close()

	if err != nil {
		return nil, fmt.Errorf("Read %q: %v", name, err)
	}
	if clErr != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

type asset struct {
	bytes []byte
	info  os.FileInfo
}

type bindataFileInfo struct {
	name    string
	size    int64
	mode    os.FileMode
	modTime time.Time
}

func (fi bindataFileInfo) Name() string {
	return fi.name
}
func (fi bindataFileInfo) Size() int64 {
	return fi.size
}
func (fi bindataFileInfo) Mode() os.FileMode {
	return fi.mode
}
func (fi bindataFileInfo) ModTime() time.Time {
	return fi.modTime
}
func (fi bindataFileInfo) IsDir() bool {
	return false
}
func (fi bindataFileInfo) Sys() interface{} {
	return nil
}

var _migrations0001CreateschedulertableSql = []byte("\x1f\x8b\x08\x00\x00\x00\x00\x00\x00\xff\x8c\x91\xdf\x6e\xd3\x30\x14\xc6\xef\xf3\x14\x9f\x76\xd5\x4a\xa4\x19\x13\x13\x52\x87\x10\x65\x75\x87\x45\xea\x40\xeb\x68\x1d\x37\x91\xe7\x9c\x25\x11\x49\x1c\x6c\x67\xd5\x1e\x89\xd7\xe0\xc9\x50\xda\xad\xe3\x8f\x50\xb9\xf4\xf9\x7e\xbf\xe3\xa3\x73\xc2\x10\x8d\x22\xe7\xad\x09\xc2\x10\xa5\xf7\x9d\x9b\x46\x51\x51\xf9\xb2\xbf\x9d\x68\xd3\x44\xde\x74\x77\x96\xa8\x50\x0d\xb9\xe8\x19\x1d\xe8\xb8\xd2\xd4\x3a\xca\xd1\xb7\x39\x59\xf8\x92\xb0\xe4\x12\xf5\xbe\x3c\x7d\x6a\x38\x8d\xa2\xed\x76\x3b\x31\x1d\xb5\xce\xf4\x56\xd3\xc4\xd8\x22\x7a\xa4\x5c\xd4\x54\x3e\x7c\x7c\x0c\xc6\xa5\xe9\x1e\x6c\x55\x94\x1e\x3f\xbe\xe3\xec\xf4\xe5\x6b\x48\xd3\x61\x61\x89\x70\x35\xcc\x80\x37\xb7\x4a\x7f\xa5\x36\x7f\xe7\xef\x0a\x6d\x86\x19\xdf\x06\xc1\xe5\x8a\xcd\x24\x03\xdb\x48\x26\xd6\x3c\x11\xe0\x0b\x88\x44\x82\x6d\xf8\x5a\xae\x71\xd2\xf7\x55\x1e\x1a\xe7\xba\x93\x8b\x03\x2c\x67\xef\x63\x06\xa7\x4b\xca\xfb\x9a\xac\xc3\x28\x00\x80\x2a\x47\x9a\xf2\x39\x3e\xad\xf8\x72\xb6\xba\xc1\x47\x76\x83\x39\x5b\xcc\xd2\x58\x62\x68\x93\x15\xd4\x92\x55\x9e\xb2\xfb\x57\xa3\xf1\x8b\x9d\xd3\xaa\x86\x70\xaf\xac\x2e\x95\x1d\x9d\x9d\x9f\x8f\x77\x9f\x8b\x34\x8e\xf7\x79\x71\x24\x7f\x50\x4d\x0d\xc9\x36\xf2\x8f\xba\xf3\xca\xd3\x3f\x83\xac\x56\xce\x67\xba\x54\x6d\x41\x79\xa6\x3c\xb8\x90\xec\x8a\xad\x0e\xec\x61\xee\xd3\xbd\xb5\xe3\x9d\x56\x35\x65\xa6\xfb\x1f\x41\x5b\x52\x7e\xdf\xdb\x57\x0d\x39\xaf\x9a\x0e\xd7\x5c\x7e\x80\xe4\x4b\x86\x2f\x89\x60\x7f\xbb\x22\xb9\x7e\xda\x4b\xdf\xe5\xc7\xfd\x34\x8e\x83\xf1\xf3\x59\x52\xc1\x3f\xa7\x0c\x5c\xcc\xd9\xe6\x97\xeb\x64\xc3\x8e\xb3\xbe\xad\xbe\xf5\x84\x44\xfc\x76\xb7\x21\x1a\x5f\x04\x3f\x03\x00\x00\xff\xff\x05\x7a\x84\x4f\xcc\x02\x00\x00")

func migrations0001CreateschedulertableSqlBytes() ([]byte, error) {
	return bindataRead(
		_migrations0001CreateschedulertableSql,
		"migrations/0001-CreateSchedulerTable.sql",
	)
}

func migrations0001CreateschedulertableSql() (*asset, error) {
	bytes, err := migrations0001CreateschedulertableSqlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "migrations/0001-CreateSchedulerTable.sql", size: 716, mode: os.FileMode(420), modTime: time.Unix(1495042893, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

// Asset loads and returns the asset for the given name.
// It returns an error if the asset could not be found or
// could not be loaded.
func Asset(name string) ([]byte, error) {
	cannonicalName := strings.Replace(name, "\\", "/", -1)
	if f, ok := _bindata[cannonicalName]; ok {
		a, err := f()
		if err != nil {
			return nil, fmt.Errorf("Asset %s can't read by error: %v", name, err)
		}
		return a.bytes, nil
	}
	return nil, fmt.Errorf("Asset %s not found", name)
}

// MustAsset is like Asset but panics when Asset would return an error.
// It simplifies safe initialization of global variables.
func MustAsset(name string) []byte {
	a, err := Asset(name)
	if err != nil {
		panic("asset: Asset(" + name + "): " + err.Error())
	}

	return a
}

// AssetInfo loads and returns the asset info for the given name.
// It returns an error if the asset could not be found or
// could not be loaded.
func AssetInfo(name string) (os.FileInfo, error) {
	cannonicalName := strings.Replace(name, "\\", "/", -1)
	if f, ok := _bindata[cannonicalName]; ok {
		a, err := f()
		if err != nil {
			return nil, fmt.Errorf("AssetInfo %s can't read by error: %v", name, err)
		}
		return a.info, nil
	}
	return nil, fmt.Errorf("AssetInfo %s not found", name)
}

// AssetNames returns the names of the assets.
func AssetNames() []string {
	names := make([]string, 0, len(_bindata))
	for name := range _bindata {
		names = append(names, name)
	}
	return names
}

// _bindata is a table, holding each asset generator, mapped to its name.
var _bindata = map[string]func() (*asset, error){
	"migrations/0001-CreateSchedulerTable.sql": migrations0001CreateschedulertableSql,
}

// AssetDir returns the file names below a certain
// directory embedded in the file by go-bindata.
// For example if you run go-bindata on data/... and data contains the
// following hierarchy:
//     data/
//       foo.txt
//       img/
//         a.png
//         b.png
// then AssetDir("data") would return []string{"foo.txt", "img"}
// AssetDir("data/img") would return []string{"a.png", "b.png"}
// AssetDir("foo.txt") and AssetDir("notexist") would return an error
// AssetDir("") will return []string{"data"}.
func AssetDir(name string) ([]string, error) {
	node := _bintree
	if len(name) != 0 {
		cannonicalName := strings.Replace(name, "\\", "/", -1)
		pathList := strings.Split(cannonicalName, "/")
		for _, p := range pathList {
			node = node.Children[p]
			if node == nil {
				return nil, fmt.Errorf("Asset %s not found", name)
			}
		}
	}
	if node.Func != nil {
		return nil, fmt.Errorf("Asset %s not found", name)
	}
	rv := make([]string, 0, len(node.Children))
	for childName := range node.Children {
		rv = append(rv, childName)
	}
	return rv, nil
}

type bintree struct {
	Func     func() (*asset, error)
	Children map[string]*bintree
}
var _bintree = &bintree{nil, map[string]*bintree{
	"migrations": &bintree{nil, map[string]*bintree{
		"0001-CreateSchedulerTable.sql": &bintree{migrations0001CreateschedulertableSql, map[string]*bintree{}},
	}},
}}

// RestoreAsset restores an asset under the given directory
func RestoreAsset(dir, name string) error {
	data, err := Asset(name)
	if err != nil {
		return err
	}
	info, err := AssetInfo(name)
	if err != nil {
		return err
	}
	err = os.MkdirAll(_filePath(dir, filepath.Dir(name)), os.FileMode(0755))
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(_filePath(dir, name), data, info.Mode())
	if err != nil {
		return err
	}
	err = os.Chtimes(_filePath(dir, name), info.ModTime(), info.ModTime())
	if err != nil {
		return err
	}
	return nil
}

// RestoreAssets restores an asset under the given directory recursively
func RestoreAssets(dir, name string) error {
	children, err := AssetDir(name)
	// File
	if err != nil {
		return RestoreAsset(dir, name)
	}
	// Dir
	for _, child := range children {
		err = RestoreAssets(dir, filepath.Join(name, child))
		if err != nil {
			return err
		}
	}
	return nil
}

func _filePath(dir, name string) string {
	cannonicalName := strings.Replace(name, "\\", "/", -1)
	return filepath.Join(append([]string{dir}, strings.Split(cannonicalName, "/")...)...)
}

