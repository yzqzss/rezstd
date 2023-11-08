package main

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const oriZstdFilename = "ori.zst"
const outputZstdFilename = "output.zst"
const logFilename = "task.log"
const zstdLevel = "-22" // MAX is 22

type Task string

func (task Task) Log(msg string) {
	line := fmt.Sprintf("%s:%s", time.Now().Format("2006-01-02 15:04:05 MST"), msg)
	fmt.Println(task, "|", line)
	logFilePath := fmt.Sprintf("./www_pub/tasks/%s/%s", task, logFilename)
	// update the log file if exists
	logFile, err := os.OpenFile(logFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Println(err.Error())
	}
	logFile.WriteString(line + "\n")
	logFile.Close()
}

func (task Task) getLog() string {
	logFilePath := fmt.Sprintf("./www_pub/tasks/%s/%s", task, logFilename)
	// update the log file if exists
	logFile, err := os.OpenFile(logFilePath, os.O_RDONLY, 0644)
	if err != nil {
		fmt.Println(err.Error())
	}
	buf := make([]byte, 1024)
	n, err := logFile.Read(buf)
	if err != nil {
		fmt.Println(err.Error())
	}
	logFile.Close()
	return string(buf[:n])
}

func main() {
	r := gin.Default()

	// Handle file download
	r.GET("/rezstd/download/:taskname/:any_filename", func(c *gin.Context) {
		taskName := c.Param("taskname")
		filePath := fmt.Sprintf("./www_pub/tasks/%s/download/%s", taskName, outputZstdFilename)
		c.File(filePath)
	})

	// Handle task status
	r.GET("/rezstd/status/:task", func(c *gin.Context) {
		task := Task(c.Param("task"))
		logFilePath := fmt.Sprintf("./www_pub/tasks/%s/%s", task, logFilename)

		// Check if the log file exists
		_, err := os.Stat(logFilePath)
		if os.IsNotExist(err) {
			c.Status(http.StatusNotFound)
			c.JSON(http.StatusNotFound, gin.H{"error": "Task not found"})
			return
		}

		// Check if the task is finished
		if _, err := os.Stat(fmt.Sprintf("./www_pub/tasks/%s/download/%s", task, outputZstdFilename)); err == nil {
			// finished := true
			// return the user log file if the task is finished (not json)
			c.String(http.StatusOK, task.getLog())
		} else {
			c.JSON(http.StatusTeapot, gin.H{"status": "running"})
		}

	})

	// Handle the file upload
	r.POST("/rezstd/upload/one", func(c *gin.Context) {

		task := Task(fmt.Sprintf("task_%s_%s", time.Now().Format("2006-01-02"), uuid.New().String()))
		taskDir := fmt.Sprintf("./www_pub/tasks/%s", task)
		// Create the task directory
		if err := os.MkdirAll(taskDir, os.ModePerm); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create task directory"})
			return
		}
		task.Log("Task created")

		// Create a log file for the task

		// Save the uploaded file
		oriFilePath := path.Join(taskDir, oriZstdFilename)
		task.Log("Saving the received file")
		file, _ := c.FormFile("file")
		if err := c.SaveUploadedFile(file, oriFilePath); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save the file"})
			task.Log("Failed to save the file")
			return
		}
		go startTask(task)

		c.JSON(http.StatusOK, gin.H{"tas": task})
	})

	r.Run()
}

func startTask(task Task) {
	taskDir := fmt.Sprintf("./www_pub/tasks/%s", task)
	oriFilePath := fmt.Sprintf("./www_pub/tasks/%s/%s", task, oriZstdFilename)

	// Run zstd to test file size
	task.Log("Testing file integrity and size")
	cmd := exec.Command("zstd", "-d", "--long=31", oriFilePath, "--stdout")
	stdout_bin, err := cmd.StdoutPipe()
	if err != nil {
		task.Log("Failed to get stdout pipe")
		return
	}
	task.Log("Calculating uncompressed size")
	// calculate the uncompressed size
	if err := cmd.Start(); err != nil {
		task.Log("Failed to start zstd")
		return
	}
	var uncompressedSize int64 = 0
	for {
		buf := make([]byte, 1024)
		n, err := stdout_bin.Read(buf)
		if err != nil {
			break
		}
		uncompressedSize += int64(n)
	}
	task.Log(fmt.Sprintf("Uncompressed size: %d", uncompressedSize))

	if err := cmd.Wait(); err != nil {
		task.Log("Failed to wait for zstd")
		return
	}

	// Recompress the file
	task.Log("Recompressing the file, building pipelines")
	tempFilePath := fmt.Sprintf("%s/%s", taskDir, "__temp_running.zst.tmp")
	dec_cmd := exec.Command("zstd", "-d", "--long=31", oriFilePath, "--stdout")
	dec_stdout_bin, err := dec_cmd.StdoutPipe()
	if err != nil {
		task.Log("Failed to get stdout pipe (decompress)")
		return
	}
	com_cmd := exec.Command("zstd", "-T0", "-v", "--compress", "--force", "--long=31", zstdLevel, "--ultra", fmt.Sprintf("--stream-size=%d", uncompressedSize), "-o", tempFilePath)
	com_stdin_bin, err := com_cmd.StdinPipe()
	if err != nil {
		task.Log("Failed to get stdin pipe (compress))")
		return
	}
	if err := com_cmd.Start(); err != nil {
		task.Log("Failed to start zstd (compress)")
		return
	}
	if err := dec_cmd.Start(); err != nil {
		task.Log("Failed to start zstd (decompress))")
		return
	}
	task.Log("Pipelines built, starting recompression")
	for {
		buf := make([]byte, 1024)
		n, err := dec_stdout_bin.Read(buf)
		if err != nil {
			break
		}
		com_stdin_bin.Write(buf[:n])
	}
	if err := com_stdin_bin.Close(); err != nil {
		task.Log("Failed to close stdin pipe (compress)")
		return
	}

	dec_err := dec_cmd.Wait()
	com_err := com_cmd.Wait()

	if dec_err != nil || com_err != nil {
		if dec_err != nil {
			task.Log("Failed to wait for zstd (decompress)")
		}
		if com_err != nil {
			task.Log("Failed to wait for zstd (compress)")
		}
		return
	}
	task.Log("Recompression finished")

	fileInfo, err := os.Stat(tempFilePath)
	if err != nil {
		task.Log("Failed to get recompressed file size")
	}
	recompressedSize := fileInfo.Size()
	task.Log(fmt.Sprintf("Recompressed file size: %d", recompressedSize))

	// Move the recompressed file to the download directory
	task.Log("Moving the recompressed file to download directory")
	downloadDir := fmt.Sprintf("%s/download", taskDir)
	if err := os.MkdirAll(downloadDir, os.ModePerm); err != nil {
		task.Log("Failed to create download directory")
		return
	}
	downloadFilePath := fmt.Sprintf("%s/%s", downloadDir, outputZstdFilename)
	if err := os.Rename(tempFilePath, downloadFilePath); err != nil {
		task.Log("Failed to move the recompressed file to the download directory")
		return
	}

	task.Log("Task finished, Great!")
}
