package azure

import (
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/AB-Lindex/go-resthelp"
	"github.com/patrickmn/go-cache"
)

// GET https://graph.microsoft.com/v1.0/servicePrincipals(appId='{{appid}}')
//   ?$select=id,displayName,appId,appOwnerOrganizationId,servicePrincipalType

var userCache = cache.New(5*time.Minute, 10*time.Minute)

var base = resthelp.New(
	resthelp.WithBaseURL("https://graph.microsoft.com/v1.0"),
)

func getApp(appId, token string) map[string]string {
	key := fmt.Sprintf("%s:%s", appId, token)

	if x, found := userCache.Get(key); found {
		slog.Debug("azure: reusing app from cache", "appId", appId)
		return x.(map[string]string)
	}

	slog.Debug("azure: fetching app from ms-graph", "appId", appId)

	graph, err := base.Get(fmt.Sprintf("servicePrincipals(appId='%s')", appId))
	if err != nil {
		return nil
	}

	graph.AddHeader("Authorization", "Bearer "+token)
	graph.AddQuery("$select", "id,displayName,appId,appOwnerOrganizationId,servicePrincipalType")
	resp, err := graph.Do()
	if err != nil {
		return nil
	}

	result := make(map[string]string)
	err = resp.ParseJSON(&result)
	if err != nil {
		return nil
	}

	var toDelete []string
	for k := range result {
		if strings.HasPrefix(k, "@") {
			toDelete = append(toDelete, k)
		}
	}
	for _, k := range toDelete {
		delete(result, k)
	}

	userCache.SetDefault(key, result)
	return result
}
