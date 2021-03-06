package handler

import (
	"fmt"
	model2 "github.com/NetEase-Object-Storage/nos-golang-sdk/model"
	"github.com/NetEase-Object-Storage/nos-golang-sdk/nosclient"
	"github.com/gin-gonic/gin"
	"github.com/wq1019/cloud_disk/errors"
	"github.com/wq1019/cloud_disk/handler/middleware"
	"github.com/wq1019/cloud_disk/model"
	"github.com/wq1019/cloud_disk/service"
	"net/http"
	"strconv"
)

type folderHandler struct {
	nosClient  *nosclient.NosClient
	bucketName string
}

// RenameFolder godoc
// @Tags 目录
// @Summary 重命名目录
// @Description 通过目录 ID 重命名目录
// @ID rename-folder
// @Accept json,multipart/form-data
// @Produce json,multipart/form-data
// @Param folder_id query uint64 true "所属的目录 ID" Format(uint64)
// @Param current_folder_id query uint64 true "当前目录 ID" Format(uint64)
// @Param new_name query string true "新的目录名" Format(string)
// @Success 204
// @Failure 404 {object} errors.GlobalError "目录不存在"
// @Failure 500 {object} errors.GlobalError
// @Router /folder/rename [PUT]
func (*folderHandler) RenameFolder(c *gin.Context) {
	l := struct {
		FolderId        int64  `json:"folder_id" form:"folder_id"`
		NewName         string `json:"new_name" form:"new_name"`
		CurrentFolderId int64  `json:"current_folder_id" form:"current_folder_id"`
	}{}
	if err := c.ShouldBind(&l); err != nil {
		_ = c.Error(err)
		return
	}
	authId := middleware.UserId(c)
	folder, err := service.LoadFolder(c.Request.Context(), l.FolderId, authId, false)
	if err != nil {
		_ = c.Error(err)
		return
	}
	err = service.RenameFolder(c.Request.Context(), folder.Id, l.CurrentFolderId, l.NewName)
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.Status(http.StatusNoContent)
}

// LoadFolder godoc
// @Tags 目录
// @Summary 加载指定的目录及子目录和文件列表
// @Description 加载指定的目录及子目录和文件列表
// @ID load-folder
// @Accept json,multipart/form-data
// @Produce json,multipart/form-data
// @Param folder_id query uint64 true "目录 ID" Format(uint64)
// @Success 200 {object} model.Folder
// @Failure 404 {object} errors.GlobalError "目录不存在 | 没有访问权限 | id 格式不正确"
// @Failure 500 {object} errors.GlobalError
// @Router /folder [GET]
func (*folderHandler) LoadFolder(c *gin.Context) {
	l := struct {
		FolderId int64 `json:"folder_id" form:"folder_id"`
	}{}
	if err := c.ShouldBind(&l); err != nil {
		_ = c.Error(errors.BadRequest("id 格式不正确", err))
		return
	}
	authId := middleware.UserId(c)
	folder, err := service.LoadFolder(c.Request.Context(), l.FolderId, authId, true)
	if err != nil {
		_ = c.Error(err)
		return
	}
	if authId != folder.UserId {
		_ = c.Error(errors.Unauthorized("没有访问权限"))
		return
	}
	c.JSON(200, folder)
}

// CreateFolder godoc
// @Tags 目录
// @Summary 创建一个目录
// @Description 创建一个目录
// @ID create-folder
// @Accept json,multipart/form-data
// @Produce json,multipart/form-data
// @Param parent_id query uint64 true "父级目录的 ID" Format(uint64)
// @Param folder_name query string true "新目录的名称" Format(string)
// @Success 201 {object} model.Folder
// @Failure 404 {object} errors.GlobalError "目录名称不能为空 | (父)目录不存在 | 目录已经存在"
// @Success 401 {object} errors.GlobalError "请先登录"
// @Failure 500 {object} errors.GlobalError
// @Router /folder [POST]
func (*folderHandler) CreateFolder(c *gin.Context) {
	l := struct {
		ParentId   int64  `json:"parent_id" form:"parent_id"`
		FolderName string `json:"folder_name" form:"folder_name"`
	}{}
	if err := c.ShouldBind(&l); err != nil {
		_ = c.Error(err)
		return
	}
	if l.FolderName == "" {
		_ = c.Error(errors.BadRequest("目录名称不能为空"))
		return
	}
	authId := middleware.UserId(c)
	parentFolder, err := service.LoadFolder(c.Request.Context(), l.ParentId, authId, false)
	if err != nil {
		_ = c.Error(err)
		return
	}
	isExist := service.ExistFolder(c.Request.Context(), authId, l.ParentId, l.FolderName)
	if isExist {
		_ = c.Error(errors.BadRequest("目录已经存在"))
		return
	}
	pId2String := strconv.FormatInt(parentFolder.Id, 10)
	folder := model.Folder{
		UserId:     authId,
		Level:      parentFolder.Level + 1,
		ParentId:   l.ParentId,
		Key:        parentFolder.Key + pId2String + model.FolderKeyPrefix,
		FolderName: l.FolderName,
	}
	err = service.CreateFolder(c.Request.Context(), &folder)
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusCreated, folder)
}

// DeleteSource godoc
// @Tags 资源
// @Summary 批量删除资源(文件/目录)
// @Description 批量删除资源(文件/目录)
// @ID delete-source
// @Accept json
// @Produce json
// @Param current_folder_id query uint64 true "当前目录的 ID"
// @Param file_ids query array false "要删除的文件 ids"
// @Param folder_ids query array false "要删除的目录 ids"
// @Success 204
// @Failure 404 {object} errors.GlobalError "请指定要删除的文件或者目录ID | 当前目录不存在"
// @Success 401 {object} errors.GlobalError "请先登录"
// @Failure 500 {object} errors.GlobalError
// @Router /folder [DELETE]
func (f *folderHandler) DeleteSource(c *gin.Context) {
	l := struct {
		FileIds         []int64 `json:"file_ids" form:"file_ids"`
		FolderIds       []int64 `json:"folder_ids" form:"folder_ids"`
		CurrentFolderId int64   `json:"current_folder_id" form:"current_folder_id"`
	}{}
	if err := c.ShouldBind(&l); err != nil {
		_ = c.Error(err)
		return
	}
	if len(l.FileIds) == 0 && len(l.FolderIds) == 0 {
		_ = c.Error(errors.BadRequest("请指定要删除的文件或者目录ID"))
		return
	}
	deleteMultiObjects := model2.DeleteMultiObjects{
		Quiet: false, //详细和静默模式，设置为 true 的时候，只返回删除错误的文件列表，设置为 false 的时候，成功和失败的文件列表都返回
	}
	authId := middleware.UserId(c)
	// 删除指定的文件
	if len(l.FileIds) > 0 {
		// 判断当前目录有没有权限
		currentFolder, err := service.LoadFolder(c.Request.Context(), l.CurrentFolderId, authId, false)
		if err != nil {
			_ = c.Error(err)
			return
		}
		hashList, err := service.DeleteFile(c.Request.Context(), l.FileIds, currentFolder.Id)
		if err != nil {
			_ = c.Error(err)
			return
		}
		for _, hash := range hashList {
			deleteMultiObjects.Append(model2.DeleteObject{Key: hash[:2] + "/" + hash[2:]})
		}
	}
	// 删除目录列表
	if len(l.FolderIds) > 0 {
		hashList, err := service.DeleteFolder(c.Request.Context(), l.FolderIds, authId)
		if err != nil {
			_ = c.Error(err)
			return
		}
		for _, hash := range hashList {
			deleteMultiObjects.Append(model2.DeleteObject{Key: hash[:2] + "/" + hash[2:]})
		}
	}

	if len(deleteMultiObjects.Objects) > 0 {
		deleteRequest := &model2.DeleteMultiObjectsRequest{
			Bucket:        f.bucketName,
			DelectObjects: &deleteMultiObjects,
		}
		_, err := f.nosClient.DeleteMultiObjects(deleteRequest)
		if err != nil {
			_ = c.Error(errors.BadRequest(fmt.Sprintf("删除文件失败: %+v", err), err))
			return
		}
	}
	c.Status(http.StatusNoContent)
}

func (*folderHandler) Move2Folder(c *gin.Context) {
	l := struct {
		FileIds      []int64 `json:"file_ids" form:"file_ids"`
		FolderIds    []int64 `json:"folder_ids" form:"folder_ids"`
		FromFolderId int64   `json:"from_folder_id" form:"from_folder_id"`
		ToFolderId   int64   `json:"to_folder_id" form:"to_folder_id"`
	}{}
	if err := c.ShouldBind(&l); err != nil {
		_ = c.Error(err)
		return
	}
	if len(l.FileIds) == 0 && len(l.FolderIds) == 0 {
		_ = c.Error(errors.BadRequest("请指定要移动的文件或者目录ID"))
		return
	}
	if l.ToFolderId == 0 {
		_ = c.Error(errors.BadRequest("请指定移动到哪个目录"))
		return
	}
	if l.FromFolderId == l.ToFolderId {
		_ = c.Error(errors.BadRequest("当前文件夹和目的文件夹相等"))
		return
	}
	authId := middleware.UserId(c)
	fromFolder, err := service.LoadFolder(c.Request.Context(), l.FromFolderId, authId, false)
	if err != nil {
		_ = c.Error(err)
		return
	}
	toFolder, err := service.LoadFolder(c.Request.Context(), l.ToFolderId, authId, false)
	if err != nil {
		_ = c.Error(err)
		return
	}
	if fromFolder.UserId != authId || toFolder.UserId != authId {
		_ = c.Error(errors.Unauthorized("没有权限移动"))
		return
	}
	if len(l.FolderIds) > 0 {
		err := service.MoveFolder(c.Request.Context(), toFolder, l.FolderIds)
		if err != nil {
			_ = c.Error(err)
			return
		}
	}
	if len(l.FileIds) > 0 {
		err := service.MoveFile(c.Request.Context(), fromFolder.Id, toFolder.Id, l.FileIds)
		if err != nil {
			_ = c.Error(err)
			return
		}
	}
	c.Status(http.StatusOK)
}

func (*folderHandler) Copy2Folder(c *gin.Context) {
	l := struct {
		FileIds      []int64 `json:"file_ids" form:"file_ids"`
		FolderIds    []int64 `json:"folder_ids" form:"folder_ids"`
		ToFolderId   int64   `json:"to_folder_id" form:"to_folder_id"`
		FromFolderId int64   `json:"from_folder_id" form:"from_folder_id"`
	}{}
	if err := c.ShouldBind(&l); err != nil {
		_ = c.Error(err)
		return
	}
	if len(l.FileIds) == 0 && len(l.FolderIds) == 0 {
		_ = c.Error(errors.BadRequest("请指定要复制的文件或者目录ID"))
		return
	}
	if l.ToFolderId == 0 {
		_ = c.Error(errors.BadRequest("请指定复制到哪个目录"))
		return
	}
	if l.FromFolderId == l.ToFolderId {
		_ = c.Error(errors.BadRequest("当前文件夹和目的文件夹相等"))
		return
	}
	var (
		totalFileSize    uint64
		allowCopyFileIds []int64
		authId           = middleware.UserId(c)
	)
	// 判断将要复制到的目录是否属于自己
	toFolder, err := service.LoadFolder(c.Request.Context(), l.ToFolderId, authId, false)
	if err != nil {
		_ = c.Error(err)
		return
	}
	// 判断指定的当前目录是否属于自己
	fromFolder, err := service.LoadFolder(c.Request.Context(), l.FromFolderId, authId, false)
	if err != nil {
		_ = c.Error(err)
		return
	}
	if toFolder.UserId != authId || fromFolder.UserId != authId {
		_ = c.Error(errors.Unauthorized("该目录没有权限复制"))
		return
	}

	if len(l.FileIds) > 0 {
		// 过滤出有权限复制的文件
		ownFiles, err := service.LoadFolderFilesByFolderIdAndFileIds(c.Request.Context(), l.FromFolderId, l.FileIds, authId)
		if err != nil {
			_ = c.Error(err)
			return
		}
		for _, file := range ownFiles {
			allowCopyFileIds = append(allowCopyFileIds, file.FileId)
		}
		// 复制当前目录指定的文件到指定目录
		if len(allowCopyFileIds) > 0 {
			totalSize, err := service.CopyFile(c.Request.Context(), l.FromFolderId, toFolder.Id, allowCopyFileIds)
			if err != nil {
				_ = c.Error(err)
				return
			}
			totalFileSize += totalSize
		}
	}
	if len(l.FolderIds) > 0 {
		// 过滤出有权限复制的目录
		ownFolders, err := service.ListFolder(c.Request.Context(), l.FolderIds, authId)
		if err != nil {
			_ = c.Error(err)
			return
		}
		// 复制指定的目录包括目录中的文件到指定位置
		if len(ownFolders) > 0 {
			totalSize, err := service.CopyFolder(c.Request.Context(), toFolder, ownFolders)
			if err != nil {
				_ = c.Error(err)
				return
			}
			totalFileSize += totalSize
		}
	}
	if totalFileSize > 0 {
		err = service.UserUpdateUsedStorage(c.Request.Context(), authId, totalFileSize, model.OperatorAdd)
		if err != nil {
			_ = c.Error(err)
			return
		}
	}

	c.Status(http.StatusOK)
}

func NewFolderHandler(client *nosclient.NosClient, bucketName string) *folderHandler {
	return &folderHandler{nosClient: client, bucketName: bucketName}
}
