package controllers

import (
	"github.com/gin-gonic/gin"
	"github.com/sdslabs/gasper/configs"
	"github.com/sdslabs/gasper/lib/mongo"
	"github.com/sdslabs/gasper/lib/redis"
	"github.com/sdslabs/gasper/lib/utils"
	"github.com/sdslabs/gasper/types"
)

// GetAllUsers gets all the users registered on the app
func GetAllUsers(c *gin.Context) {
	queries := c.Request.URL.Query()
	filter := utils.QueryToFilter(queries)
	// Convert `admin` field from string to boolean
	if filter[mongo.AdminKey] == "true" {
		filter[mongo.AdminKey] = true
	} else if filter[mongo.AdminKey] == "false" {
		filter[mongo.AdminKey] = false
	}
	c.JSON(200, gin.H{
		"success": true,
		"data":    mongo.FetchUserInfo(filter),
	})
}

func changeUserPrivilege(c *gin.Context, admin bool) {
	user := c.Param("user")
	filter := types.M{
		mongo.EmailKey: user,
	}
	update := types.M{
		mongo.AdminKey: admin,
	}

	err := mongo.UpdateUser(filter, update)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			c.AbortWithStatusJSON(400, gin.H{
				"success": false,
				"error":   "No such user exists",
			})
		} else {
			utils.SendServerErrorResponse(c, err)
		}
		return
	}
	c.JSON(200, gin.H{
		"success": true,
	})
}

// GrantSuperuserPrivilege grants superuser access to a user
func GrantSuperuserPrivilege(c *gin.Context) {
	changeUserPrivilege(c, true)
}

// RevokeSuperuserPrivilege revokes superuser access from a user
func RevokeSuperuserPrivilege(c *gin.Context) {
	changeUserPrivilege(c, false)
}

// GetAllNodes fetches all the nodes registered on redis corresponding to their service
func GetAllNodes(c *gin.Context) {
	services := configs.ServiceMap
	res := gin.H{}
	// loop just to get names of services dynamically
	for service := range services {
		instances, err := redis.FetchServiceInstances(service)
		if err != nil {
			utils.SendServerErrorResponse(c, err)
			return
		}
		res[service] = instances
	}
	res["success"] = true
	c.JSON(200, res)
}

// GetNodesByName fetches master nodes for 'master' and others for 'workers'
// Rest specific service nodes are returned
func GetNodesByName(c *gin.Context) {
	node := c.Param("type")
	res := gin.H{}
	switch node {
	case WorkerNode:
		services := configs.ServiceMap
		for service := range services {
			if service == types.Master {
				continue
			}
			instances, err := redis.FetchServiceInstances(service)
			if err != nil {
				utils.SendServerErrorResponse(c, err)
				return
			}
			res[service] = instances
		}
		res["success"] = true
		c.JSON(200, res)
		return
	case MasterNode:
		node = types.Master
	default:
		services := configs.ServiceMap
		serviceExists := false
		for service := range services {
			if node == service {
				serviceExists = true
			}
		}
		if !serviceExists {
			c.AbortWithStatusJSON(400, gin.H{
				"success": false,
				"error":   "service does not exist",
			})
			return
		}
	}
	instances, err := redis.FetchServiceInstances(node)
	if err != nil {
		utils.SendServerErrorResponse(c, err)
		return
	}
	res[node] = instances
	res["success"] = true
	c.JSON(200, res)
}

// DeleteUserByAdmin deletes the user from database
func DeleteUserByAdmin(c *gin.Context) {
	deleteUser(c, c.Param("user"))
}
