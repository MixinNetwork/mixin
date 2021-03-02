package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/mholt/archiver"
)

func main() {
	bin := flag.String("bin", "/tmp/bin", "the mixin binary directory")
	flag.Parse()

	for {
		err := updateBinary(*bin)
		if err != nil {
			log.Println(err)
		}
		time.Sleep(10 * time.Minute)
	}
}

func updateBinary(bin string) error {
	f, err := os.OpenFile(bin+"/VERSION", os.O_RDONLY|os.O_CREATE, 0666)
	if err != nil {
		return err
	}
	err = f.Close()
	if err != nil {
		return err
	}

	version, err := os.ReadFile(bin + "/VERSION")
	if err != nil {
		return err
	}
	log.Printf("OLD VERSION %s\n", string(version))

	resp, err := http.Get("https://api.github.com/repos/MixinNetwork/mixin/releases/latest")
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var body struct {
		Author struct {
			Login string `json:"login"`
		}
		Assets []struct {
			Name               string `json:"name"`
			UpdatedAt          string `json:"updated_at"`
			BrowserDownloadURL string `json:"browser_download_url"`
		} `json:"assets"`
	}
	err = json.NewDecoder(resp.Body).Decode(&body)
	if err != nil {
		return err
	}
	log.Println("LATEST VERSION", body)

	if body.Author.Login != "cedricfung" {
		return fmt.Errorf("hacked release author %s", body.Author.Login)
	}
	if len(body.Assets) != 1 {
		return fmt.Errorf("invalid release assets number %d", len(body.Assets))
	}
	asset := body.Assets[0]
	if strings.TrimSpace(string(version)) == asset.UpdatedAt {
		return fmt.Errorf("same version found %s", asset.UpdatedAt)
	}
	if !strings.HasPrefix(asset.Name, "mixin-linux-x64-v") || !strings.HasSuffix(asset.Name, ".tar.bz2") {
		return fmt.Errorf("invalid asset format %s", asset.Name)
	}

	tar := "/tmp/mixin.tar.bz2"
	err = download(asset.BrowserDownloadURL, tar)
	if err != nil {
		return err
	}

	name := asset.Name[:len(asset.Name)-len(".tar.bz2")]
	err = extract(tar, name)
	if err != nil {
		return err
	}

	err = os.Rename(bin+"/mixin", bin+"/mixin.old")
	if err != nil {
		return err
	}
	err = os.Rename("/tmp/"+name+"/mixin", bin+"/mixin")
	if err != nil {
		return err
	}

	err = os.WriteFile(bin+"/VERSION", []byte(asset.UpdatedAt), 0644)
	if err != nil {
		return err
	}
	err = exec.Command("sudo", "systemctl", "restart", "mixin.service").Run()
	if err != nil {
		return err
	}
	log.Printf("NEW VERSION %s DEPLOYED\n", asset.UpdatedAt)
	return nil
}

func extract(tar, name string) error {
	err := archiver.Unarchive(tar, "/tmp")
	if err != nil {
		return err
	}

	xin := "/tmp/" + name + "/mixin"
	return os.Chmod(xin, 0755)
}

func download(src, dst string) error {
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	resp, err := http.Get(src)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("invalid response %d", resp.StatusCode)
	}

	_, err = io.Copy(out, resp.Body)
	return err
}
