package main

import (
	"fmt"
	"log"
	"os"
	"path"
	"sync"
)

func calcPermissions(filePermissions os.FileMode) os.FileMode {
	owner := filePermissions & 0700
	return owner + owner>>3 + owner>>6
}

func adjustPermissionsAsync(fpath string, permissions os.FileMode, waitGroup *sync.WaitGroup, results chan<- string) {
	newPerm := calcPermissions(permissions)
	if newPerm != permissions {
		var result string
		if err := os.Chmod(fpath, newPerm); err == nil {
			result = fmt.Sprintf("%s %#o -> %#o", fpath, permissions, newPerm)
		} else {
			result = err.Error()
		}
		results <- result
	}
	waitGroup.Done()
}

func adjustPermissions(fpath string, waitGroup *sync.WaitGroup, results chan<- string) {
	fi, err := os.Stat(fpath)
	if err != nil {
		log.Fatal(err)
	}
	waitGroup.Add(1)
	go adjustPermissionsAsync(fpath, fi.Mode().Perm(), waitGroup, results)
}

func enumDir(dirPath string, waitGroup *sync.WaitGroup, results chan<- string) {
	dir, err := os.Open(dirPath)
	if err != nil {
		log.Fatal(err)
	}
	defer dir.Close()

	files, err := dir.Readdir(0)

	for _, f := range files {
		fullPath := path.Join(dirPath, f.Name())
		if f.IsDir() {
			waitGroup.Add(1)
			go enumDir(fullPath, waitGroup, results)
		}

		waitGroup.Add(1)
		go adjustPermissionsAsync(fullPath, f.Mode().Perm(), waitGroup, results)
	}
	waitGroup.Done()
}

func main() {
	if len(os.Args) < 2 {
		log.Fatalf("Usage: %s [directory]\n", os.Args[0])
	}

	results := make(chan string, 20)

	waitGroup := sync.WaitGroup{}

	baseDir := os.Args[1]
	adjustPermissions(baseDir, &waitGroup, results)

	waitGroup.Add(1)
	go enumDir(baseDir, &waitGroup, results)

	go func() {
		waitGroup.Wait()
		close(results)
	}()

	var total uint64

	for str := range results {
		total++
		fmt.Println(str)
	}
	fmt.Println("total:", total)
}
