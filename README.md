# merge
merge is a simple command line tool that combines the real-time output of multiple processes together in a single terminal. merge saves you from having to open a new terminal tab/tmux pane for each long-running service you need running in the background (for example a local webpack, HTTP, and database server in order to develop a web application).

## Example usage
```sh
merge './manage.py runserver' 'webpack --watch' 'redis-server --port 1337'
```

## Demonstration
![Demonstration GIF](https://user-images.githubusercontent.com/35482043/76673656-15d3c300-6575-11ea-9ee1-781b4580b24b.gif)

## Getting started
1. Install [Go](https://golang.org/) if you don't already have it
2. After cloning the repository, run `go build`
3. Use `./merge` (you'll probably want to symlink this into your `$PATH`)
