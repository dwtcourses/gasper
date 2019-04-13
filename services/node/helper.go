package node

import (
	"github.com/sdslabs/SWS/lib/api"
	"github.com/sdslabs/SWS/lib/configs"
	"github.com/sdslabs/SWS/lib/docker"
	"github.com/sdslabs/SWS/lib/types"
	"github.com/sdslabs/SWS/lib/utils"
)

// installPackages function installs the dependancies for the app
func installPackages(appEnv *types.ApplicationEnv) (string, types.ResponseError) {
	cmd := []string{"bash", "-c", `export NVM_DIR="$HOME/.nvm" && [ -s "$NVM_DIR/nvm.sh" ] && \. "$NVM_DIR/nvm.sh"; npm install &> /proc/1/fd/1`}
	execID, err := docker.ExecDetachedProcess(appEnv.Context, appEnv.Client, appEnv.ContainerID, cmd)
	if err != nil {
		return "", types.NewResErr(500, "Failed to perform npm install in the container", err)
	}
	return execID, nil
}

// startApp function starts the app using pm2
func startApp(index string, appEnv *types.ApplicationEnv) (string, types.ResponseError) {
	cmd := []string{"bash", "-c", `export NVM_DIR="$HOME/.nvm" && [ -s "$NVM_DIR/nvm.sh" ] && \. "$NVM_DIR/nvm.sh"; pm2 start ` + index + ` &> /proc/1/fd/1`}
	execID, err := docker.ExecDetachedProcess(appEnv.Context, appEnv.Client, appEnv.ContainerID, cmd)
	if err != nil {
		return "", types.NewResErr(500, "Failed to perform start app in the container", err)
	}
	return execID, nil
}

func pipeline(data map[string]interface{}) types.ResponseError {
	context := data["context"].(map[string]interface{})
	appConf := &types.ApplicationConfig{
		DockerImage:  utils.ServiceConfig["node"].(map[string]interface{})["image"].(string),
		ConfFunction: configs.CreateNodeContainerConfig,
	}

	appEnv, resErr := api.SetupApplication(appConf, data)
	if resErr != nil {
		return resErr
	}

	var execID string
	// Perform npm install in the container
	if data["npm"].(bool) {
		execID, resErr = installPackages(appEnv)
		if resErr != nil {
			return resErr
		}
		data["execID"] = execID
	}

	index := context["index"].(string)

	// Start app using pm2 in the container
	execID, resErr = startApp(index, appEnv)
	if resErr != nil {
		return resErr
	}
	data["execID"] = execID

	return nil
}
