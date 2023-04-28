package inventory

import (
	"fmt"
	"regexp"
	"strings"

	v1 "k8s.io/api/core/v1"
)

// Compile the regexes used for parsing once so they can be reused without having to recompile
var (
	digestRegex = regexp.MustCompile(`@(sha[[:digit:]]{3}:[[:alnum:]]{32,})`)
	tagRegex    = regexp.MustCompile(`:[\w][\w.-]{0,127}$`)
)

func getContainersInPod(pod v1.Pod, missingTagPolicy, dummyTag string) []Container {
	// Look at both status/spec for init and regular containers
	// Must use status when looking at containers in order to obtain the container ID
	// from the Status and the Image tag from the Spec
	containers := make(map[string]Container, 0)

	processPodSpec := func(c v1.Container) {
		if containerFound, ok := containers[c.Name]; ok {
			containerFound.ImageTag = strings.Split(c.Image, "@")[0]
			containerFound.PodUID = string(pod.UID)
		} else {
			containers[c.Name] = Container{
				PodUID:   string(pod.UID),
				ImageTag: strings.Split(c.Image, "@")[0],
				Name:     c.Name,
			}
		}
	}
	processPodStatus := func(c v1.ContainerStatus) {
		repo := c.ImageID
		digest := ""
		digestresult := digestRegex.FindStringSubmatchIndex(repo)
		if len(digestresult) > 0 {
			i := digestresult[0]
			digest = repo[i+1:]
		}

		if containerFound, ok := containers[c.Name]; ok {
			containerFound.ID = c.ContainerID
			containerFound.ImageDigest = digest
			containers[c.Name] = containerFound
		} else {
			containers[c.Name] = Container{
				ID:          c.ContainerID,
				PodUID:      string(pod.UID),
				ImageTag:    strings.Split(c.Image, "@")[0],
				ImageDigest: digest,
				Name:        c.Name,
			}
		}
	}

	for _, c := range pod.Spec.InitContainers {
		processPodSpec(c)
	}
	for _, c := range pod.Status.InitContainerStatuses {
		processPodStatus(c)
	}
	for _, c := range pod.Spec.Containers {
		processPodSpec(c)
	}
	for _, c := range pod.Status.ContainerStatuses {
		processPodStatus(c)
	}

	var containerList []Container
	for _, c := range containers {
		tagFound := tagRegex.FindStringSubmatchIndex(c.ImageTag)
		if len(tagFound) == 0 {
			switch missingTagPolicy {
			case "dummy":
				c.ImageTag = fmt.Sprintf("%s:%s", c.ImageTag, dummyTag)
			case "digest":
				digest := strings.Split(c.ImageDigest, ":")
				c.ImageTag = fmt.Sprintf("%s:%s", c.ImageTag, digest[len(digest)-1])
			}
		}

		containerList = append(containerList, c)
	}
	return containerList
}

func GetContainersFromPods(pods []v1.Pod, ignoreNotRunning bool, missingTagPolicy, dummyTag string) []Container {
	var containers []Container

	for _, pod := range pods {
		if ignoreNotRunning && pod.Status.Phase != v1.PodRunning {
			continue
		}
		containers = append(containers, getContainersInPod(pod, missingTagPolicy, dummyTag)...)
	}

	// Handle missing tags
	var finalContainers []Container
	for _, c := range containers {
		tagFound := tagRegex.FindStringSubmatchIndex(c.ImageTag)
		if len(tagFound) == 0 && missingTagPolicy == "drop" {
			continue
		}
		finalContainers = append(finalContainers, c)
	}

	return finalContainers
}
