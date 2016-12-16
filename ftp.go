// FTP server for downloading code to the devices
package main

import (
    "os/user"
    "crypto/tls"
    "errors"
    "io/ioutil"
    "os"
    "fmt"
	ftp "github.com/fclairamb/ftpserver/server"
)

// TeletypeDriver defines a very basic serverftp driver
type TeletypeDriver struct {
    baseDir   string
    tlsConfig *tls.Config
}

func (driver *TeletypeDriver) WelcomeUser(cc ftp.ClientContext) (string, error) {
    cc.SetDebug(true)
    return "Welcome to TTSERVE", nil
}

func (driver *TeletypeDriver) AuthUser(cc ftp.ClientContext, user, pass string) (ftp.ClientHandlingDriver, error) {
    if user == "bad" || pass == "bad" {
        return nil, errors.New("BAD username or password !")
    } else {
        return driver, nil
    }
}

func (driver *TeletypeDriver) GetTLSConfig() (*tls.Config, error) {
    if driver.tlsConfig == nil {
        fmt.Printf("FTP: Loading certificate\n")
        usr, _ := user.Current()
        directory := usr.HomeDir
        directory = directory + TTServerFTPCertPath
        if cert, err := tls.LoadX509KeyPair(directory+"/mycert.crt", directory+"/mycert.key"); err == nil {
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

func (driver *TeletypeDriver) ChangeDirectory(cc ftp.ClientContext, directory string) error {
    _, err := os.Stat(driver.baseDir + directory)
    return err
}

func (driver *TeletypeDriver) MakeDirectory(cc ftp.ClientContext, directory string) error {

    // Teletype NOT IMPLEMENTED, because our FTP server is read-only root-only open-to-all
    return errors.New("MKDIR not implemented")

    return os.Mkdir(driver.baseDir+directory, 0777)
}

func (driver *TeletypeDriver) ListFiles(cc ftp.ClientContext) ([]os.FileInfo, error) {

    path := driver.baseDir + cc.Path()

    files, err := ioutil.ReadDir(path)

    return files, err
}

func (driver *TeletypeDriver) UserLeft(cc ftp.ClientContext) {

}

func (driver *TeletypeDriver) OpenFile(cc ftp.ClientContext, path string, flag int) (ftp.FileStream, error) {

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

func (driver *TeletypeDriver) GetFileInfo(cc ftp.ClientContext, path string) (os.FileInfo, error) {
    path = driver.baseDir + path

    return os.Stat(path)
}

func (driver *TeletypeDriver) CanAllocate(cc ftp.ClientContext, size int) (bool, error) {
    return true, nil
}

func (driver *TeletypeDriver) ChmodFile(cc ftp.ClientContext, path string, mode os.FileMode) error {

    // Teletype NOT IMPLEMENTED, because our FTP server is read-only root-only open-to-all
    return errors.New("CHMOD not implemented")

    path = driver.baseDir + path
    return os.Chmod(path, mode)
}

func (driver *TeletypeDriver) DeleteFile(cc ftp.ClientContext, path string) error {

    // Teletype NOT IMPLEMENTED, because our FTP server is read-only root-only open-to-all
    return errors.New("RM not implemented")

    path = driver.baseDir + path
    return os.Remove(path)
}

func (driver *TeletypeDriver) RenameFile(cc ftp.ClientContext, from, to string) error {

    // Teletype NOT IMPLEMENTED, because our FTP server is read-only root-only open-to-all
    return errors.New("MV not implemented")

    from = driver.baseDir + from
    to = driver.baseDir + to
    return os.Rename(from, to)
}

func (driver *TeletypeDriver) GetSettings() *ftp.Settings {
    config := &ftp.Settings{}
	config.PublicHost = ""
    config.ListenHost = ""
    config.ListenPort = TTServerPortFTP
    config.MaxConnections = 10000
    return config
}

// Create a new instance of an FTP driver
func NewTeletypeDriver() *TeletypeDriver {
    usr, _ := user.Current()
    directory := usr.HomeDir
    directory = directory + TTServerBuildPath
    driver := &TeletypeDriver{}
    driver.baseDir = directory
    return driver
}
