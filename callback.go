package media_library

import (
	"encoding/json"
	"errors"
	"mime/multipart"

	"github.com/jinzhu/gorm"
	"github.com/qor/serializable_meta"
)

func cropField(field *gorm.Field, scope *gorm.Scope) (cropped bool) {
	if field.Field.CanAddr() {
		if media, ok := field.Field.Addr().Interface().(MediaLibrary); ok && !media.Cropped() {
			option := parseTagOption(field.Tag.Get("media_library"))
			if media.GetFileHeader() != nil || media.NeedCrop() {
				var file multipart.File
				var err error
				if fileHeader := media.GetFileHeader(); fileHeader != nil {
					file, err = media.GetFileHeader().Open()
				} else {
					file, err = media.Retrieve(media.URL("original"))
				}

				if err != nil {
					scope.Err(err)
					return false
				}

				media.Cropped(true)

				if url := media.GetURL(option, scope, field, media); url == "" {
					scope.Err(errors.New("invalid URL"))
				} else {
					result, _ := json.Marshal(map[string]string{"Url": url})
					media.Scan(string(result))
				}

				if file != nil {
					defer file.Close()
					var handled = false
					for _, handler := range mediaLibraryHandlers {
						if handler.CouldHandle(media) {
							if scope.Err(handler.Handle(media, file, option)) == nil {
								handled = true
							}
						}
					}

					// Save File
					if !handled {
						scope.Err(media.Store(media.URL(), option, file))
					}
				}
				return true
			}
		}
	}
	return false
}

func SaveAndCropImage(isCreate bool) func(scope *gorm.Scope) {
	return func(scope *gorm.Scope) {
		if !scope.HasError() {
			var updateColumns = map[string]interface{}{}

			if value, ok := scope.Value.(serializable_meta.SerializableMetaInterface); ok {
				newScope := scope.New(value.GetSerializableArgument(value))
				for _, field := range newScope.Fields() {
					if cropField(field, scope) && isCreate {
						updateColumns["value"], _ = json.Marshal(newScope.Value)
					}
				}
			}

			for _, field := range scope.Fields() {
				if cropField(field, scope) && isCreate {
					updateColumns[field.DBName] = field.Field.Interface()
				}
			}

			if !scope.HasError() && len(updateColumns) != 0 {
				scope.NewDB().Model(scope.Value).UpdateColumns(updateColumns)
			}
		}
	}
}

func RegisterCallbacks(db *gorm.DB) {
	db.Callback().Update().Before("gorm:before_update").Register("media_library:save_and_crop", SaveAndCropImage(false))
	db.Callback().Create().After("gorm:after_create").Register("media_library:save_and_crop", SaveAndCropImage(true))
}
