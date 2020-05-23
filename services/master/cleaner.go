package master

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/sdslabs/gasper/configs"
	"github.com/sdslabs/gasper/lib/factory"
	"github.com/sdslabs/gasper/lib/mongo"
	"github.com/sdslabs/gasper/lib/redis"
	"github.com/sdslabs/gasper/lib/utils"
	"github.com/sdslabs/gasper/types"
)

// rescheduleApplications re-deploys applications present on lost nodes to other least loaded nodes
func rescheduleApplications(apps []types.M) {
	if len(apps) == 0 {
		return
	}

	// distributionArray stores the number of more apps each instance can hold
	// scoreArray and URLArray stores the urls of the instances and the corresponding urls
	var distributionArray, scoreArray []int
	var URLArray []string

	n := int64(len(apps))

	// fetch n least loaded appmaker instances
	instances, err := redis.GetLeastLoadedInstancesWithScores(redis.WorkerInstanceKey, n)
	if err != nil {
		utils.LogError(err)
		return
	}

	if len(instances) == 0 {
		utils.LogError(errors.New("No instances available for re-scheduling"))
		return
	}

	for _, instance := range instances {
		scoreArray = append(scoreArray, int(instance.Score))
		distributionArray = append(distributionArray, 0)
		URLArray = append(URLArray, fmt.Sprintf("%v", instance.Member))
	}

	deployedApps := n
	level := scoreArray[0]

	// generate the distributionArray based the corresponding scores(loads)
	for deployedApps > 0 {
		for idx := range scoreArray {
			if level < scoreArray[idx] {
				level++
				break
			} else {
				deployedApps--
				scoreArray[idx]++
				distributionArray[idx]++
			}
		}
	}

	var index int

	// deploy the apps based on the distribution generated by the distributionArray
	for _, app := range apps {
		instanceURL := URLArray[index]
		distributionArray[index]--
		if distributionArray[index] <= 0 {
			index++
		}
		dataBytes, err := json.Marshal(app)
		if err != nil {
			utils.LogError(err)
			continue
		}
		name, ok := app[mongo.NameKey].(string)
		if !ok {
			continue
		}
		language, ok := app[mongo.LanguageKey].(string)
		if !ok {
			continue
		}
		owner, ok := app[mongo.OwnerKey].(string)
		if !ok {
			continue
		}
		utils.LogInfo("Re-scheduling application %s to %s", name, instanceURL)

		// TODO :-
		// 1. Shift the below function call to a goroutine worker pool i.e fixed number of goroutines
		// (maybe equal to number of CPU logical cores) to avoid CPU overload and thrashing
		// 2. Check for errors and if any, reschedule that application to a different instance
		go factory.CreateApplication(language, owner, instanceURL, dataBytes)
	}
}

// inspectInstance checks whether a given instance is alive or not and deletes that instance
// if it is dead
func inspectInstance(service, instance string) {
	// Handle GenDNS's health-check by sending a UDP probe instead of TCP
	if service == types.GenDNS {
		if !utils.IsGenDNSAlive(instance) {
			if err := redis.RemoveServiceInstance(service, instance); err != nil {
				utils.LogError(err)
			}
		}
		return
	}
	if utils.NotAlive(instance) {
		if err := redis.RemoveServiceInstance(service, instance); err != nil {
			utils.LogError(err)
		}
		// Re-schedule applications for AppMaker microservice
		if service == types.AppMaker {
			if !strings.Contains(instance, ":") {
				utils.LogError(fmt.Errorf("Instance %s is in invalid format", instance))
				return
			}
			instanceIP := strings.Split(instance, ":")[0]
			apps := mongo.FetchAppInfo(types.M{
				mongo.HostIPKey: instanceIP,
			})
			go rescheduleApplications(apps)
		}
	}
}

// removeDeadServiceInstances removes all inactive instances in a given service
func removeDeadServiceInstances(service string) {
	instances, err := redis.FetchServiceInstances(service)
	if err != nil {
		utils.LogError(err)
	}
	for _, instance := range instances {
		go inspectInstance(service, instance)
	}
}

// removeDeadInstances removes all inactive instances in every service
func removeDeadInstances() {
	time.Sleep(5 * time.Second)
	for service := range configs.ServiceMap {
		go removeDeadServiceInstances(service)
	}
}

// ScheduleCleanup runs removeDeadInstances on given intervals of time
func ScheduleCleanup() {
	time.Sleep(10 * time.Second)
	interval := configs.ServiceConfig.Master.CleanupInterval * time.Second
	scheduler := utils.NewScheduler(interval, removeDeadInstances)
	scheduler.RunAsync()
}