package main

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"github.com/tidwall/gjson"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
)

var definitions = make(map[string]interface{})

func main() {
	if len(os.Args) < 3 {
		fmt.Println("postmantoswagger [source file] [target file]")
		os.Exit(0)
	}

	source_file := os.Args[1]
	target_file := os.Args[2]

	source_content, err := readContent(source_file)

	if err != nil {
		fmt.Println(err.Error())
		os.Exit(0)
	}

	//监测目标文件是否存在
	write, err := checkTargetIsExists(target_file)
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(0)
	}
	m, ok := gjson.ParseBytes(source_content).Value().(map[string]interface{})
	if !ok {
		fmt.Println("failed")
		os.Exit(0)
	}

	translate(m, write)

}

func readContent(path string) ([]byte, error) {
	_, err := CheckIsExists(path)
	if err != nil {
		return nil, err
	}

	file, err := os.OpenFile(path, os.O_RDONLY, 0755)
	if err != nil {
		return nil, err
	}
	return ioutil.ReadAll(file)

}

func checkTargetIsExists(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0755)
}

func translate(source map[string]interface{}, wfp *os.File) {
	write_content := make(map[string]interface{})
	write_content["swagger"] = "2.0"
	write_content["info"] = initInfo(source["info"].(map[string]interface{}))
	write_content["host"] = ""

	contents := initContents(source["item"].([]interface{}))
	write_content["tags"] = getDefault(contents, "tags", []interface{}{})
	write_content["schemes"] = initSchemes()

	write_content["paths"] = getDefault(contents, "paths", struct{}{})
	write_content["definitions"] = definitions

	json_content, err := json.Marshal(write_content)
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(0)
	}

	wfp.Write(json_content)
	wfp.Close()
}

func CheckIsExists(path string) (bool, error) {
	if _, err := os.Stat(path); err != nil {
		//is not exists
		if os.IsNotExist(err) {
			return false, fmt.Errorf("%s is not exists", path)
		} else {
			return false, err
		}
	} else {
		//exists
		return true, nil
	}
}

func initInfo(info map[string]interface{}) map[string]interface{} {
	info_content := make(map[string]interface{})
	info_content["description"] = getDefault(info, "description", "")
	info_content["version"] = "1.0.0"
	info_content["title"] = getDefault(info, "name", "title")
	info_content["contact"] = make(map[string]interface{})
	info_content["contact"] = initContact()
	return info_content
}

func initContact() map[string]interface{} {
	contact := make(map[string]interface{})
	return contact
}

func initContents(items []interface{}) map[string]interface{} {
	var tags []interface{}
	default_tag := make(map[string]string)
	default_tag["description"] = "default tag"
	default_tag["name"] = "default"
	tags = append(tags, default_tag)
	paths := make(map[string]interface{})
	paths, tags = initItems(items, paths, tags)

	result := make(map[string]interface{})
	result["tags"] = tags
	result["paths"] = paths

	return result
}

func initTag(item map[string]interface{}) map[string]interface{} {
	tag := make(map[string]interface{})
	tag["name"] = getDefault(item, "name", "")
	tag["description"] = getDefault(item, "description", "")
	return tag
}

func initSchemes() []string {
	var scheme []string
	scheme = append(scheme, "https")
	scheme = append(scheme, "http")
	return scheme
}

func initItems(items []interface{}, paths map[string]interface{}, tags []interface{}) (map[string]interface{}, []interface{}) {
	for _, item_temp := range items {
		item := item_temp.(map[string]interface{})
		isExists := getDefault(item, "item", false)
		switch isExists.(type) {
		case bool:
			request := item["request"].(map[string]interface{})
			method := strings.ToLower(request["method"].(string))
			url := request["url"].(map[string]interface{})
			key := initPostmanUrl(url["path"].([]interface{}))
			temp_key := make(map[string]interface{})
			temp_key[method] = initPathContent(item, "default", paths)
			//已存在
			if temp_path, ok := paths[key].(map[string]interface{}); ok {
				temp_path[method] = temp_key[method]
				paths[key] = temp_path
			} else {
				paths[key] = temp_key
			}
		default: //文件夹内的
			tags = append(tags, initTag(item))
			paths, tags = initPaths(isExists.([]interface{}), getDefault(item, "name", "").(string), paths, tags)
		}
	}
	return paths, tags
}

func initPaths(items []interface{}, tag string, paths map[string]interface{}, tags []interface{}) (map[string]interface{}, []interface{}) {
	for _, item_temp := range items {
		item := item_temp.(map[string]interface{})
		//监测是否还有下一级
		isExists := getDefault(item, "item", false)
		switch isExists.(type) {
		case bool:
			request := item["request"].(map[string]interface{})
			method := strings.ToLower(request["method"].(string))
			url := request["url"].(map[string]interface{})
			key := initPostmanUrl(url["path"].([]interface{}))
			temp_key := make(map[string]interface{})
			temp_key[method] = initPathContent(item, tag, paths)
			//已存在
			if temp_path, ok := paths[key].(map[string]interface{}); ok {
				temp_path[method] = temp_key[method]
				paths[key] = temp_path
			} else {
				paths[key] = temp_key
			}
		default:
			paths, tags = initPaths(isExists.([]interface{}), tag, paths, tags)
		}

	}

	return paths, tags
}

func initPostmanUrl(paths []interface{}) string {
	length := len(paths)
	result := ""
	for i := 0; i < length; i++ {
		path := paths[i].(string)
		result += "/" + path
	}
	return result
}

func initParameters(key string, body map[string]interface{}) ([]map[string]interface{}, bool) {
	count := strings.Count(key, "{")
	temp_index := 0

	var parameters []map[string]interface{}
	is_formdata := false
	for i := 0; i < count; i++ {
		start := strings.Index(key[temp_index:], "{")
		end := strings.Index(key[temp_index:], "}")
		value := key[start+1+temp_index : end+temp_index]

		parameters_temp := make(map[string]interface{})
		parameters_temp["name"] = value
		parameters_temp["in"] = "path"
		parameters_temp["description"] = ""
		parameters_temp["required"] = true
		parameters_temp["type"] = "integer"
		parameters_temp["format"] = "int64"
		parameters = append(parameters, parameters_temp)
		temp_index = end + 1
	}
	if body["mode"] != nil {
		mode := body["mode"].(string)
		switch body[mode].(type) {
		case []interface{}:
			temp_parameters := body[mode].([]interface{})
			for _, temp_parameter := range temp_parameters {
				temp := temp_parameter.(map[string]interface{})
				parameters_temp := make(map[string]interface{})
				parameters_temp["name"] = temp["key"]
				parameters_temp["in"] = "formData"
				parameters_temp["description"] = getDefault(temp, "description", "")
				parameters_temp["type"] = "string"
				parameters = append(parameters, parameters_temp)
			}
		}
		is_formdata = true
	}

	return parameters, is_formdata
}

func getDefault(source map[string]interface{}, index string, default_value interface{}) interface{} {
	if value, isExists := source[index]; isExists {
		return value
	} else {
		return default_value
	}
}

func initPathContent(item map[string]interface{}, tag string, paths map[string]interface{}) map[string]interface{} {
	request := item["request"].(map[string]interface{})
	url := request["url"].(map[string]interface{})
	key := initPostmanUrl(url["path"].([]interface{}))
	temp_key := make(map[string]interface{})
	method := strings.ToLower(request["method"].(string))
	if _, exists := paths[key]; exists {
		temp_key = paths[key].(map[string]interface{})
		temp_key[method] = make(map[string]interface{})
	} else {
		temp_key = make(map[string]interface{})
		temp_key[method] = make(map[string]interface{})
	}
	tags := []string{tag}
	temp_method := make(map[string]interface{})
	temp_method["tags"] = tags
	temp_method["summary"] = item["name"].(string)
	temp_method["description"] = getDefault(item, "description", "")
	temp_method["consumes"] = []string{"application/json"}
	temp_method["produces"] = []string{"application/json"}
	//
	var is_formdata bool
	temp_method["parameters"], is_formdata = initParameters(key, request["body"].(map[string]interface{}))
	if len(temp_method["parameters"].([]map[string]interface{})) == 0 {
		temp_method["parameters"] = make([]interface{}, 0)
	}
	if is_formdata {
		temp_method["consumes"] = []string{"application/x-www-form-urlencoded"}
	}
	response := item["response"].([]interface{})
	if len(response) != 0 {
		temp_method["responses"] = initResponse(response, key)
	} else {
		temp_method["responses"] = new(struct{})
	}

	return temp_method
}

func initResponse(responses []interface{}, path string) map[string]interface{} {
	result_responses := make(map[string]interface{})

	for _, response_temp := range responses {
		response := response_temp.(map[string]interface{})
		result_response := make(map[string]interface{})
		key := strconv.Itoa(int(response["code"].(float64)))
		body := response["body"].(string)
		if body != "" {
			//解析
			response_body, ok := gjson.Parse(body).Value().(map[string]interface{})
			if !ok {
				//fmt.Println("parse body failed:" + body)
				//os.Exit(0)
				response_body = make(map[string]interface{})
				response_body["body"] = body
			}

			temp_key := []byte(response["name"].(string))
			response_key := fmt.Sprintf("%x", md5.Sum(temp_key))

			//response_key := strings.Replace(strings.Replace(path, "/", "", -1) + response["name"].(string), " ", "", -1);
			temp := initDefinitions(response_body, response_key)
			definitions[response_key] = temp
			schema := make(map[string]interface{})
			schema["$ref"] = "#/definitions/" + response_key
			result_response["schema"] = schema
		}
		result_response["description"] = response["name"]
		result_responses[key] = result_response
	}
	return result_responses
}

func initDefinitions(response map[string]interface{}, key string) map[string]interface{} {
	result := make(map[string]interface{})
	result["type"] = "object"
	properties := make(map[string]interface{})
	temp := make(map[string]interface{})
	temp["type"] = "string"
	properties["msg"] = temp
	temp["type"] = "integer"
	temp["format"] = "int64"
	properties["errcode"] = temp

	data := initResponseData(response["data"], key)
	properties["data"] = data

	result["properties"] = properties
	return result
}

func initResponseData(data interface{}, iden_key string) map[string]interface{} {
	result := make(map[string]interface{})
	switch data.(type) {
	case map[string]interface{}:
		result["$ref"] = initResponseDataObject(data, iden_key)

	case []interface{}:
		result["type"] = "array"
		temp_data := data.([]interface{})
		if len(temp_data) == 0 {
			result["items"] = make(map[string]interface{})
		} else {
			result["items"] = initResponseData(data.([]interface{})[0], iden_key)
		}
	}

	return result
}

func initResponseDataObject(data interface{}, iden_key string) string {
	response := data.(map[string]interface{})
	properties := make(map[string]interface{})
	definitions_temp := make(map[string]interface{})
	definitions_temp["type"] = "object"
	for key, value := range response {
		temp_key := make(map[string]interface{})
		switch value.(type) {
		case float32:
			temp_key["type"] = "integer"
			temp_key["format"] = "int64"
		case float64:
			temp_key["type"] = "integer"
			temp_key["format"] = "int64"
		case int:
			temp_key["type"] = "integer"
			temp_key["format"] = "int64"
		default:
			temp_key["type"] = "string"
		}
		properties[key] = temp_key
		definitions_temp["properties"] = properties
	}
	def_key := iden_key + "data"
	definitions[def_key] = definitions_temp

	return "#/definitions/" + def_key

}

//func getInType(postman string) string {
//	in_type := make(map[string]string)
//	in_type["formdata"] = "formData"
//}
