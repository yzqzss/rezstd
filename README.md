# Re-Zstandard

A simple HTTP server that can re-compress Zstandard files user uploaded with ultra compression level of 22 and max window size, then provide a download link.

Why I made this?

1. I want my files to get the best compression ratio.
2. Most files I want to compressed are HTML/XML, so the gain of ultra compression level is significant.
3. Most of my devices don't have enough CPU/MEM/TIME to do ultra compression.
4. Files are tiny, Internet is fast, latency is not a problem.
5. Let's choose a server to do the job.

## REQUIREMENTS

- [Zstandard](https://github.com/facebook/zstd) (>= 1.5.5)
- 64-bit OS
- **10GB RAM+ (16GB+ recommended)**

## HOW TO USE

upload a file to the server:

```bash
$ curl -X POST http://localhost:8080/rezstd/upload/one   -F "file=@parts.igem.org_wiki-20231102-history.xml.zst"
{"task":"task_2023-11-08_211a5d65-4229-4eb2-91bf-3ac68fe45e5e"}
```

"task_2023-11-08_211a5d65-4229-4eb2-91bf-3ac68fe45e5e" is the task id.

wait for the task to finish:

```bash
$ curl -X GET http://localhost:8080/rezstd/status/{task_id}
{"log-last-line":null,"status":"not found"} # HTTP 404
or
{"log-last-line":"2023-11-12 03:17:22.13104278 +0800:Pipelines built, starting recompression","status":"running"} # HTTP 418
or
{"log-last-line":"2023-11-12 03:19:16.59214542 +0800:Task finished, Great!","status":"finished"} # HTTP 200
```

then download the re-compressed file:

```bash
wget http://localhost:8080/rezstd/download/{task_id}/the_file_name_you_want.zst
```

Check the log if you want after the task is created:

```bash
curl http://localhost:8080/rezstd/log/{task_id}
```

## Configuration

by environment variables:

```bash
export GIN_MODE=release # set to release to disable debug mode
export PORT=8080 # port to listen, default 8080
```

## TODO

- [ ] HTTP DELETE to delete a task
- [ ] Customizable compression level and wlog size
- [ ] Auto delete tasks after a period of time
- [ ] Queue tasks
