// FTP server for downloading code to the devices
package main

import (
    "os/user"
    "crypto/tls"
    "errors"
    "github.com/fclairamb/ftpserver/server"
    "io/ioutil"
    "os"
	"fmt"
)

// TeletypeDriver defines a very basic serverftp driver
type TeletypeDriver struct {
    baseDir   string
    tlsConfig *tls.Config
}

func (driver *TeletypeDriver) WelcomeUser(cc server.ClientContext) (string, error) {
    cc.SetDebug(true)
    return "Welcome to TTSERVE", nil
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

    _, err := os.Stat(driver.baseDir + directory)
    return err
}

func (driver *TeletypeDriver) MakeDirectory(cc server.ClientContext, directory string) error {

	// Teletype NOT IMPLEMENTED, because our FTP server is read-only root-only open-to-all
	return errors.New("MKDIR not implemented")

    return os.Mkdir(driver.baseDir+directory, 0777)
}

func (driver *TeletypeDriver) ListFiles(cc server.ClientContext) ([]os.FileInfo, error) {

    path := driver.baseDir + cc.Path()

    files, err := ioutil.ReadDir(path)

    return files, err
}

func (driver *TeletypeDriver) UserLeft(cc server.ClientContext) {

}

func (driver *TeletypeDriver) OpenFile(cc server.ClientContext, path string, flag int) (server.FileStream, error) {

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
	config.Host = ""
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
