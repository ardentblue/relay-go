# relay-go

relay-go SDK is a go library for interacting with Relay. For full documentation visit [api-docs.relaypro.com](https://api-docs.relaypro.com)

## Usage

The following code snippet demonstrates a very simple "Hello World" workflow. However, it does show some of the power that is available through the Relay SDK.
A complete example can be found in `examples/server/` folder.

```go
	http.HandleFunc("/helloworld", func(w http.ResponseWriter, r *http.Request) {
		conn, _, _, err := ws.UpgradeHTTP(r, w)
		if err != nil {
			return
		}
		defer conn.Close()

		stopC := make(chan struct{})
		sendC := make(chan []byte, 5)
		recvC := make(chan []byte, 5)

		go func() {
			relay, err := relaygo.NewRelayDevice(sendC, recvC, stopC)
			if err != nil {
				log.Fatal("failed to initialize relay", err)
				return
			}

			relay.Vibrate()
			deviceName, _ := relay.GetName()
			relay.Say("What is your name?")
			user, _ := relay.Listen([]string{})

			response := fmt.Sprintf("Hello %s! Your device name is %s", user, deviceName)
			relay.Say(response)
			relay.Terminate()

		}()
		handleWS(conn, recvC, sendC, stopC)
	})
```

Features demonstrated here:

* A vibration command is sent to the device.
* The workflow then uses text-to-speech to prompt the user for their name.
* The workflow awaits for a response from the device user.
* The workflow then again uses text-to-speech to reply with a dynamic message.
* Finally, the workflow is terminated and the device is returned to its original state.

In this sample, a workflow callback function is registered with the name `helloworld`. This value
of `helloworld` is used to map a WebSocket connection at the path `ws://yourhost:port/helloworld`
to the registered workflow callback function.

## Workflow Registration

  More thorough documentation on how toregister your workflow on a Relay device
  can be found at https://api-docs.relaypro.com/docs/register-workflows

## Development

```bash
git clone git@github.com:ardentblue/relay-go.git
cd relay-go
go mod download
```

To run the existing example in `examples/server`
```bash
cd examples/server
go build
./server --addr 0.0.0.0:8081
```

## License
[MIT](https://choosealicense.com/licenses/mit/)
