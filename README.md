# Server Manager (SM)

![Go](https://img.shields.io/badge/go-%2300ADD8.svg?style=for-the-badge&logo=go&logoColor=white)
![HTML5](https://img.shields.io/badge/html5-%23E34F26.svg?style=for-the-badge&logo=html5&logoColor=white)
![TypeScript](https://img.shields.io/badge/typescript-%233178C6.svg?style=for-the-badge&logo=typescript&logoColor=white)
![CSS3](https://img.shields.io/badge/css3-%231572B6.svg?style=for-the-badge&logo=css3&logoColor=white)
![Docker](https://img.shields.io/badge/docker-%230db7ed.svg?style=for-the-badge&logo=docker&logoColor=white)
![Vue.js](https://img.shields.io/badge/vue.js-%234FC08D.svg?style=for-the-badge&logo=vuedotjs&logoColor=white)
![TailwindCSS](https://img.shields.io/badge/tailwindcss-%2338B2AC.svg?style=for-the-badge&logo=tailwind-css&logoColor=white)


SM is a very flexible web interface used to manage a dedicated Assetto Corsa server. With SM, everything is a preset and can be re-used: cars, weather patterns, sessions and difficulty.

Features:

 - Supports CSP ([Custom Shaders Patch](https://acstuff.club/patch/))
 - Supports both Windows and Linux
 - No installation required

To be added:
 - Support for UDP Plugins

# How to use

- Launch the compiled or [pre-built](https://github.com/8bitmcu/ServerManager/releases) executable
- If a browser tab did not appear, manually navigate to http://localhost:3030
- Log in with the username `admin` and password `admin`
- Fill in the Server Configuration page. Some defaults have been provided
    * Remember to open the TCP, UDP and HTTP Ports in your firewall or modem
    * Do not open the web ui's 3030 port
- Follow the instructions on the Content and Mods page
- Create, in any order, at least one difficulty preset, one session preset, one time & weather preset and one car class
- Create a new Event Category with at least one Event
- In the Queue page, add at least one Event to the Queue
- You can now start the server. You can monitor the server from the Server Status page


## Running on Linux

### Prebuilt binaries

Use the pre-built binaries provided in the release tab.

### Dockerfile

The following instructions will build and run the application using the Dockerfile

1. Clone the repository locally
2. cd in the directory and build the Dockerfile
3. Run the application in Docker
4. Acceess the Web UI using http://localhost:3030

```sh
git clone https://github.com/8bitmcu/ServerManager.git
cd ServerManager
docker build . --tag 'servermanager'
docker run --network=host 'servermanager' -v /path/to/corsa:/corsa
```

### Docker compose

1. Clone the repository locally
2. cd into the directory and run `docker compose up`
3. Access the Web UI using https://localhost:443, http://localhost:80 or http://localhost:3030

## Running on Windows

Use the pre-built binaries provided in the release tab.

## Building the project

The project builds the Vue SPA first and embeds the production output with
native Go `embed`. npm is required for the webapp build.
Run the following to install the dependencies:

```sh
make deps
```

### Compiling for Linux

It is best to use the included Makefile to generate a build

```sh
make build
```

### Local dev mode

For faster UI iteration, you can run the app in local dev mode without rebuilding the Go binary on every UI change.

```sh
make rundebug
```

This runs the app with `-debug`, which serves:

- the built SPA from `src/embed/webapp/dist`
- static files from `src/embed` under `/static`
- config templates like `src/embed/ini/server_cfg.ini` and `src/embed/ini/entry_list.ini` from disk

Notes:

- run it from the repository root
- Vue/CSS changes require rebuilding the SPA, or use `make webapp-dev` for Vite's dev server
- image and INI template changes under `src/embed` are picked up after restarting the app

### Docker dev mode

If you do not want to install Go locally, you can run the same debug workflow in Docker:

```sh
make rundebug-docker
```

This starts a dev container that:

- mounts the repository into `/go/src/app`
- runs the app with `-debug`
- serves static files and INI templates directly from `src/embed`
- exposes the Web UI on `http://localhost:3030`

To stop it:

```sh
make stopdebug-docker
```

### Cross-compile for Windows

The makefile provides an easy to use method to create a build for Windows

```
make buildwin
```

# Screenshots

![server screen](assets/server.png)

![content screen](assets/content.png)

![difficulty screen](assets/difficulty.png)

![difficulty2 screen](assets/difficulty2.png)

![session screen](assets/session.png)

![time screen](assets/time.png)

![class screen](assets/class.png)
