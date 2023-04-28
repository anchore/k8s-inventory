package inventory

import (
	"regexp"
	"strings"

	v1 "k8s.io/api/core/v1"
)

// Compile the regexes used for parsing once so they can be reused without having to recompile
var (
	digestRegex = regexp.MustCompile(`@(sha[[:digit:]]{3}:[[:alnum:]]{32,})`)
	tagRegex    = regexp.MustCompile(`:[\w][\w.-]{0,127}$`)
)

func getImageTagFromContainer(image string) string {
	tag := ""
	minusSha := strings.Split(image, "@")[0]
	tagresult := tagRegex.FindStringSubmatchIndex(minusSha)
	if len(tagresult) > 0 {
		tag = minusSha
	}
	return tag
}

func getContainersInPod(pod v1.Pod) []Container {
	// Look at both status/spec for init and regular containers
	// Must use status when looking at containers in order to obtain the container ID
	// from the Status and the Image tag from the Spec
	containers := make(map[string]Container, 0)

	processPodSpec := func(c v1.Container) {
		tag := getImageTagFromContainer(c.Image)
		if containerFound, ok := containers[c.Name]; ok {
			containerFound.ImageTag = tag
			containerFound.PodUID = string(pod.UID)
		} else {
			containers[c.Name] = Container{
				PodUID:   string(pod.UID),
				ImageTag: tag,
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
				ImageTag:    getImageTagFromContainer(c.Image),
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
		containerList = append(containerList, c)
	}
	return containerList
}

func GetContainersFromPods(pods []v1.Pod) []Container {
	var containers []Container

	for _, pod := range pods {
		containers = append(containers, getContainersInPod(pod)...)
	}

	return containers
}
