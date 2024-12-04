# The Gift List

This small repository contains a simple website that shows a list of presents for friends and family to buy!

## Running the app

There are two ways of running this locally.

### Debugging with Go Air

To test the project with hot reload I'm using [Go Air](https://github.com/air-verse/air)

- Clone the repository to your machine.
- Get all the packages with `go mod tidy`
- Run the command `air`

### Docker image

Just build and run docker with:

- `docker build -t <imagename>`
- `docker run -p <port>:<port> <imagename>`
