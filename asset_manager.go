package media_library

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"regexp"

	"github.com/jinzhu/gorm"
	"github.com/qor/qor/admin"
	"github.com/qor/qor/resource"
)

type AssetManager struct {
	gorm.Model
	File FileSystem `media_library:"URL:/system/assets/{{primary_key}}/{{filename_with_hash}}"`
}

func (*AssetManager) ConfigureQorResource(res resource.Resourcer) {
	if res, ok := res.(*admin.Resource); ok {
		router := res.GetAdmin().GetRouter()
		router.Post(fmt.Sprintf("^/%v/upload", res.ToParam()), func(context *admin.Context) {
			result := AssetManager{}
			result.File.Scan(context.Request.MultipartForm.File["file"])
			context.GetDB().Save(&result)
			bytes, _ := json.Marshal(map[string]string{"filelink": result.File.URL(), "filename": result.File.GetFileName()})
			context.Writer.Write(bytes)
		})

		assetURL := regexp.MustCompile(`^/system/assets/(\d+)/`)
		router.Post(fmt.Sprintf("^/%v/crop", res.ToParam()), func(context *admin.Context) {
			var err error
			var url struct{ Url string }
			defer context.Request.Body.Close()

			var buf bytes.Buffer
			io.Copy(&buf, context.Request.Body)
			if err = json.Unmarshal(buf.Bytes(), &url); err == nil {
				if matches := assetURL.FindStringSubmatch(url.Url); len(matches) > 1 {
					result := &AssetManager{}
					if err = context.GetDB().Find(result, matches[1]).Error; err == nil {
						if err = result.File.Scan(buf.Bytes()); err == nil {
							if err = context.GetDB().Save(result).Error; err == nil {
								bytes, _ := json.Marshal(map[string]string{"url": result.File.URL(), "filename": result.File.GetFileName()})
								context.Writer.Write(bytes)
								return
							}
						}
					}
				}
			}

			bytes, _ := json.Marshal(map[string]string{"err": err.Error()})
			context.Writer.Write(bytes)
		})
	}
}
