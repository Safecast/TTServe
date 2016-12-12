// FTP server for downloading code to the devices
package main

import (
    "os/user"
    "crypto/tls"
    "errors"
    "github.com/fclairamb/ftpserver/server"
    "io"
    "io/ioutil"
    "os"
	"fmt"
    "time"
)

// TeletypeDriver defines a very basic serverftp driver
type TeletypeDriver struct {
    baseDir   string
    tlsConfig *tls.Config
}

func (driver *TeletypeDriver) WelcomeUser(cc server.ClientContext) (string, error) {
    cc.SetDebug(true)
    // This will remain the official name for now
    return "Welcome on https://github.com/fclairamb/ftpserver", nil
}

func (driver *TeletypeDriver) AuthUser(cc server.ClientContext, user, pass string) (server.ClientHandlingDriver, error) {
    if user == "bad" || pass == "bad" {
        return nil, errors.New("BAD username or password !")
    } else {
        return driver, nil
    }
}

func (driver *TeletypeDriver) GetTLSConfig() (*tls.Config, error) {
    if driver.tlsConfig == nil {
		fmt.Printf("FTP: Loading certificate\n")
        if cert, err := tls.LoadX509KeyPair("sample/certs/mycert.crt", "certs/mycert.key"); err == nil {
            driver.tlsConfig = &tls.Config{
                NextProtos:   []string{"ftp"},
                Certificates: []tls.Certificate{cert},
            }
        } else {
            return nil, err
        }
    }
    return driver.tlsConfig, nil
}

func (driver *TeletypeDriver) ChangeDirectory(cc server.ClientContext, directory string) error {

	// Teletype NOT IMPLEMENTED, because our FTP server is read-only root-only open-to-all
	return errors.New("CD not implemented")

    if directory == "/debug" {
        cc.SetDebug(!cc.Debug())
        return nil
    } else if directory == "/virtual" {
        return nil
    }
    _, err := os.Stat(driver.baseDir + directory)
    return err
}

func (driver *TeletypeDriver) MakeDirectory(cc server.ClientContext, directory string) error {

	// Teletype NOT IMPLEMENTED, because our FTP server is read-only root-only open-to-all
	return errors.New("MKDIR not implemented")

    return os.Mkdir(driver.baseDir+directory, 0777)
}

func (driver *TeletypeDriver) ListFiles(cc server.ClientContext) ([]os.FileInfo, error) {

    if cc.Path() == "/virtual" {
        files := make([]os.FileInfo, 0)
        files = append(files,
            VirtualFileInfo{
                name: "localpath.txt",
                mode: os.FileMode(0666),
                size: 1024,
            },
            VirtualFileInfo{
                name: "file2.txt",
                mode: os.FileMode(0666),
                size: 2048,
            },
        )
        return files, nil
    }

    path := driver.baseDir + cc.Path()

    files, err := ioutil.ReadDir(path)

    // We add a virtual dir
    if cc.Path() == "/" && err == nil {
        files = append(files, VirtualFileInfo{
            name: "virtual",
            mode: os.FileMode(0666) | os.ModeDir,
            size: 4096,
        })
    }

    return files, err
}

func (driver *TeletypeDriver) UserLeft(cc server.ClientContext) {

}

func (driver *TeletypeDriver) OpenFile(cc server.ClientContext, path string, flag int) (server.FileStream, error) {

    if path == "/virtual/localpath.txt" {
        return &VirtualFile{content: []byte(driver.baseDir)}, nil
    }

    path = driver.baseDir + path

	// Teletype NOT IMPLEMENTED - our FTP server is read-only root-only open-to-all
	flag = os.O_RDONLY
	
    // If we are writing and we are not in append mode, we should remove the file
    if (flag & os.O_WRONLY) != 0 {
        flag |= os.O_CREATE
        if (flag & os.O_APPEND) == 0 {
            os.Remove(path)
        }
    }

    return os.OpenFile(path, flag, 0666)
}

func (driver *TeletypeDriver) GetFileInfo(cc server.ClientContext, path string) (os.FileInfo, error) {
    path = driver.baseDir + path

    return os.Stat(path)
}

func (driver *TeletypeDriver) CanAllocate(cc server.ClientContext, size int) (bool, error) {
    return true, nil
}

func (driver *TeletypeDriver) ChmodFile(cc server.ClientContext, path string, mode os.FileMode) error {

	// Teletype NOT IMPLEMENTED, because our FTP server is read-only root-only open-to-all
	return errors.New("CHMOD not implemented")

    path = driver.baseDir + path
    return os.Chmod(path, mode)
}

func (driver *TeletypeDriver) DeleteFile(cc server.ClientContext, path string) error {

	// Teletype NOT IMPLEMENTED, because our FTP server is read-only root-only open-to-all
	return errors.New("RM not implemented")

    path = driver.baseDir + path
    return os.Remove(path)
}

func (driver *TeletypeDriver) RenameFile(cc server.ClientContext, from, to string) error {

	// Teletype NOT IMPLEMENTED, because our FTP server is read-only root-only open-to-all
	return errors.New("MV not implemented")

    from = driver.baseDir + from
    to = driver.baseDir + to
    return os.Rename(from, to)
}

func (driver *TeletypeDriver) GetSettings() *server.Settings {
	config := &server.Settings{}
	config.Host = TTServerAddress
	config.Port = TTServerPortFTP
	config.MaxConnections = 10000
    return config
}

// Note: This is not a mistake. Interface can be pointers. There seems to be a lot of confusion around this in the
//       server_ftp original code.
func NewTeletypeDriver() *TeletypeDriver {
    usr, _ := user.Current()
    directory := usr.HomeDir
    directory = directory + TTServerBuildPath
    driver := &TeletypeDriver{}
	driver.baseDir = directory
    return driver
}

type VirtualFile struct {
    content    []byte // Content of the file
    readOffset int    // Reading offset
}

func (f *VirtualFile) Close() error {
    return nil
}

func (f *VirtualFile) Read(buffer []byte) (int, error) {
    n := copy(buffer, f.content[f.readOffset:])
    f.readOffset += n
    if n == 0 {
        return 0, io.EOF
    }

    return n, nil
}

func (f *VirtualFile) Seek(n int64, w int) (int64, error) {
    return 0, nil
}

func (f *VirtualFile) Write(buffer []byte) (int, error) {
    return 0, nil
}

type VirtualFileInfo struct {
    name string
    size int64
    mode os.FileMode
}

func (f VirtualFileInfo) Name() string {
    return f.name
}

func (f VirtualFileInfo) Size() int64 {
    return f.size
}

func (f VirtualFileInfo) Mode() os.FileMode {
    return f.mode
}

func (f VirtualFileInfo) IsDir() bool {
    return f.mode.IsDir()
}

func (f VirtualFileInfo) ModTime() time.Time {
    return time.Now().UTC()
}

func (f VirtualFileInfo) Sys() interface{} {
    return nil
}
