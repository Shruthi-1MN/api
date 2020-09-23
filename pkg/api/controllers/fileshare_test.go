// Copyright 2019 The OpenSDS Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package controllers

import (
	"bytes"
	ctx "context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/astaxie/beego"
	"github.com/astaxie/beego/context"
	c "github.com/sodafoundation/api/pkg/context"
	"github.com/sodafoundation/api/pkg/db"
	"github.com/sodafoundation/api/pkg/model"
	pb "github.com/sodafoundation/api/pkg/model/proto"
	"github.com/sodafoundation/api/pkg/utils/constants"
	. "github.com/sodafoundation/api/testutils/collection"
	ctrtest "github.com/sodafoundation/api/testutils/controller/testing"
	dbtest "github.com/sodafoundation/api/testutils/db/testing"
)

////////////////////////////////////////////////////////////////////////////////
//                      Prepare for mock server                               //
////////////////////////////////////////////////////////////////////////////////
func init() {
	beego.Router("/v1beta/file/shares", NewFakeFileSharePortal(),
		"post:CreateFileShare;get:ListFileShares")
	beego.Router("/v1beta/file/shares/:fileshareId", NewFakeFileSharePortal(),
		"get:GetFileShare;put:UpdateFileShare;delete:DeleteFileShare")

	beego.Router("/v1beta/file/snapshots", NewFakeFileShareSnapshotPortal(),
		"post:CreateFileShareSnapshot;get:ListFileShareSnapshots")
	beego.Router("/v1beta/file/snapshots/:snapshotId", NewFakeFileShareSnapshotPortal(),
		"get:GetFileShareSnapshot;put:UpdateFileShareSnapshot;delete:DeleteFileShareSnapshot")

	beego.Router("/v1beta/file/acls", NewFakeFileSharePortal(),
		"post:CreateFileShareAcl;get:ListFileSharesAcl")
	beego.Router("/v1beta/file/acls/:aclId", NewFakeFileSharePortal(),
		"get:GetFileShareAcl;delete:DeleteFileShareAcl")
}

func NewFakeFileSharePortal() *FileSharePortal {
	mockClient := new(ctrtest.Client)

	mockClient.On("Connect", "localhost:50049").Return(nil)
	mockClient.On("Close").Return(nil)
	mockClient.On("CreateFileShare", ctx.Background(), &pb.CreateFileShareOpts{
		Id:               "d2975ebe-d82c-430f-b28e-f373746a71ca",
		Name:             "sample-fileshare-01",
		Size:             int64(1),
		Description:      "This is first sample fileshare for testing",
		AvailabilityZone: "default",
		PoolId:           "a5965ebe-dg2c-434t-b28e-f373746a71ca",
		Context:          c.NewAdminContext().ToJson(),
		Profile:          SampleFileShareProfiles[0].ToJson(),
		ExportLocations:  []string{"192.168.100.100"},
		SnapshotId:       "b7602e18-771e-11e7-8f38-dbd6d291f4eg",
		SnapshotName:     "sample-snapshot-01",
	}).Return(&pb.GenericResponse{}, nil)
	mockClient.On("DeleteFileShare", ctx.Background(), &pb.DeleteFileShareOpts{
		Id:              "d2975ebe-d82c-430f-b28e-f373746a71ca",
		PoolId:          "a5965ebe-dg2c-434t-b28e-f373746a71ca",
		Context:         c.NewAdminContext().ToJson(),
		Profile:         SampleFileShareProfiles[0].ToJson(),
		ExportLocations: []string{"192.168.100.100"},
	}).Return(&pb.GenericResponse{}, nil)
	mockClient.On("CreateFileShareAcl", ctx.Background(), &pb.CreateFileShareAclOpts{
		Id:               "6ad25d59-a160-45b2-8920-211be282e2df",
		FileshareId:      "d2975ebe-d82c-430f-b28e-f373746a71ca",
		Description:      "This is a sample Acl for testing",
		Type:             "ip",
		AccessCapability: []string{"Read", "Write"},
		AccessTo:         "10.32.109.15",
		Profile:          SampleFileShareProfiles[0].ToJson(),
		Context:          c.NewAdminContext().ToJson(),
	}).Return(&pb.GenericResponse{}, nil)
	mockClient.On("DeleteFileShareAcl", ctx.Background(), &pb.DeleteFileShareAclOpts{
		Id:               "6ad25d59-a160-45b2-8920-211be282e2df",
		FileshareId:      "d2975ebe-d82c-430f-b28e-f373746a71ca",
		Description:      "This is a sample Acl for testing",
		Type:             "ip",
		AccessCapability: []string{"Read", "Write"},
		AccessTo:         "10.32.109.15",
		Profile:          SampleFileShareProfiles[0].ToJson(),
		Context:          c.NewAdminContext().ToJson(),
		Metadata:         map[string]string{},
	}).Return(&pb.GenericResponse{}, nil)

	return &FileSharePortal{
		CtrClient: mockClient,
	}
}

func NewFakeFileShareSnapshotPortal() *FileShareSnapshotPortal {
	mockClient := new(ctrtest.Client)
	mockClient.On("Connect", "localhost:50049").Return(nil)
	mockClient.On("Close").Return(nil)
	mockClient.On("CreateFileShareSnapshot", ctx.Background(), &pb.CreateFileShareSnapshotOpts{
		Id:          "3769855c-a102-11e7-b772-17b880d2f537",
		Name:        "sample-snapshot-01",
		Description: "This is the first sample snapshot for testing",
		FileshareId: "d2975ebe-d82c-430f-b28e-f373746a71ca",
		Context:     c.NewAdminContext().ToJson(),
		Profile:     SampleFileShareProfiles[0].ToJson(),
	}).Return(&pb.GenericResponse{}, nil)
	mockClient.On("DeleteFileShareSnapshot", ctx.Background(), &pb.DeleteFileShareSnapshotOpts{
		Id:          "3769855c-a102-11e7-b772-17b880d2f537",
		FileshareId: "d2975ebe-d82c-430f-b28e-f373746a71ca",
		Context:     c.NewAdminContext().ToJson(),
		Profile:     SampleFileShareProfiles[0].ToJson(),
	}).Return(&pb.GenericResponse{}, nil)
	return &FileShareSnapshotPortal{
		CtrClient: mockClient,
	}
}

////////////////////////////////////////////////////////////////////////////////
//                            Tests for FileShare                             //
////////////////////////////////////////////////////////////////////////////////

var (
	fakeFileShare = &model.FileShareSpec{
		BaseModel: &model.BaseModel{
			Id:        "f4a5e666-c669-4c64-a2a1-8f9ecd560c78",
			CreatedAt: "2017-10-24T16:21:32",
		},
		Name:             "fake FileShare",
		Description:      "fake FileShare",
		Size:             99,
		AvailabilityZone: "unknown",
		Status:           "available",
		PoolId:           "831fa5fb-17cf-4410-bec6-1f4b06208eef",
		ProfileId:        "d3a109ff-3e51-4625-9054-32604c79fa90",
	}
	fakeFileShares = []*model.FileShareSpec{fakeFileShare}
)

func TestListFileShares(t *testing.T) {

	t.Run("Should return 200 if everything works well", func(t *testing.T) {
		var sampleFileShares = []*model.FileShareSpec{&SampleFileShares[0], &SampleFileShares[1]}
		mockClient := new(dbtest.Client)
		m := map[string][]string{
			"offset":  {"0"},
			"limit":   {"1"},
			"sortDir": {"asc"},
			"sortKey": {"name"},
		}
		mockClient.On("ListFileSharesWithFilter", c.NewAdminContext(), m).Return(sampleFileShares, nil)
		db.C = mockClient

		r, _ := http.NewRequest("GET", "/v1beta/file/shares?offset=0&limit=1&sortDir=asc&sortKey=name", nil)
		w := httptest.NewRecorder()
		beego.InsertFilter("*", beego.BeforeExec, func(httpCtx *context.Context) {
			httpCtx.Input.SetData("context", c.NewAdminContext())
		})
		beego.BeeApp.Handlers.ServeHTTP(w, r)

		var output []*model.FileShareSpec
		json.Unmarshal(w.Body.Bytes(), &output)
		assertTestResult(t, w.Code, 200)
		assertTestResult(t, output, sampleFileShares)
	})

	t.Run("Should return 500 if list file share with bad request", func(t *testing.T) {
		mockClient := new(dbtest.Client)
		m := map[string][]string{
			"offset":  {"0"},
			"limit":   {"1"},
			"sortDir": {"asc"},
			"sortKey": {"name"},
		}
		mockClient.On("ListFileSharesWithFilter", c.NewAdminContext(), m).Return(nil, errors.New("db error"))
		db.C = mockClient

		r, _ := http.NewRequest("GET", "/v1beta/file/shares?offset=0&limit=1&sortDir=asc&sortKey=name", nil)
		w := httptest.NewRecorder()
		beego.InsertFilter("*", beego.BeforeExec, func(httpCtx *context.Context) {
			httpCtx.Input.SetData("context", c.NewAdminContext())
		})
		beego.BeeApp.Handlers.ServeHTTP(w, r)
		assertTestResult(t, w.Code, 500)
	})
}

func TestGetFileShare(t *testing.T) {

	t.Run("Should return 200 if everything works well", func(t *testing.T) {
		mockClient := new(dbtest.Client)
		mockClient.On("GetFileShare", c.NewAdminContext(), "bd5b12a8-a101-11e7-941e-d77981b584d8").Return(&SampleFileShares[0], nil)
		db.C = mockClient

		r, _ := http.NewRequest("GET", "/v1beta/file/shares/bd5b12a8-a101-11e7-941e-d77981b584d8", nil)
		w := httptest.NewRecorder()
		beego.InsertFilter("*", beego.BeforeExec, func(httpCtx *context.Context) {
			httpCtx.Input.SetData("context", c.NewAdminContext())
		})
		beego.BeeApp.Handlers.ServeHTTP(w, r)

		var output model.FileShareSpec
		json.Unmarshal(w.Body.Bytes(), &output)
		assertTestResult(t, &output, &SampleFileShares[0])
	})

	t.Run("Should return 404 if get file share replication with bad request", func(t *testing.T) {
		mockClient := new(dbtest.Client)
		mockClient.On("GetFileShare", c.NewAdminContext(), "bd5b12a8-a101-11e7-941e-d77981b584d8").Return(nil, errors.New("db error"))
		db.C = mockClient

		r, _ := http.NewRequest("GET", "/v1beta/file/shares/bd5b12a8-a101-11e7-941e-d77981b584d8", nil)
		w := httptest.NewRecorder()
		beego.InsertFilter("*", beego.BeforeExec, func(httpCtx *context.Context) {
			httpCtx.Input.SetData("context", c.NewAdminContext())
		})
		beego.BeeApp.Handlers.ServeHTTP(w, r)
		assertTestResult(t, w.Code, 404)
	})
}

func TestUpdateFileShare(t *testing.T) {
	var jsonStr = []byte(`{
		"id": "bd5b12a8-a101-11e7-941e-d77981b584d8",
		"name":"fake FileShare",
		"description":"fake Fileshare"
	}`)
	var expectedJson = []byte(`{
		"id": "bd5b12a8-a101-11e7-941e-d77981b584d8",
		"name": "fake FileShare",
		"description": "fake FileShare",
		"size": 1,
		"status": "available",
		"poolId": "084bf71e-a102-11e7-88a8-e31fe6d52248",
		"profileId": "1106b972-66ef-11e7-b172-db03f3689c9c"
	}`)
	var expected model.FileShareSpec
	json.Unmarshal(expectedJson, &expected)

	t.Run("Should return 200 if everything works well", func(t *testing.T) {
		fileshare := model.FileShareSpec{BaseModel: &model.BaseModel{}}
		json.NewDecoder(bytes.NewBuffer(jsonStr)).Decode(&fileshare)
		mockClient := new(dbtest.Client)
		mockClient.On("UpdateFileShare", c.NewAdminContext(), &fileshare).Return(&expected, nil)
		db.C = mockClient

		r, _ := http.NewRequest("PUT", "/v1beta/file/shares/bd5b12a8-a101-11e7-941e-d77981b584d8", bytes.NewBuffer(jsonStr))
		w := httptest.NewRecorder()
		r.Header.Set("Content-Type", "application/JSON")
		beego.InsertFilter("*", beego.BeforeExec, func(httpCtx *context.Context) {
			httpCtx.Input.SetData("context", c.NewAdminContext())
		})
		beego.BeeApp.Handlers.ServeHTTP(w, r)
		var output model.FileShareSpec
		json.Unmarshal(w.Body.Bytes(), &output)
		assertTestResult(t, w.Code, 200)
		assertTestResult(t, &output, &expected)
	})

	t.Run("Should return 500 if update file share with bad request", func(t *testing.T) {
		fileshare := model.FileShareSpec{BaseModel: &model.BaseModel{}}
		json.NewDecoder(bytes.NewBuffer(jsonStr)).Decode(&fileshare)
		mockClient := new(dbtest.Client)
		mockClient.On("UpdateFileShare", c.NewAdminContext(), &fileshare).Return(nil, errors.New("db error"))
		db.C = mockClient

		r, _ := http.NewRequest("PUT", "/v1beta/file/shares/bd5b12a8-a101-11e7-941e-d77981b584d8", bytes.NewBuffer(jsonStr))
		w := httptest.NewRecorder()
		r.Header.Set("Content-Type", "application/JSON")
		beego.InsertFilter("*", beego.BeforeExec, func(httpCtx *context.Context) {
			httpCtx.Input.SetData("context", c.NewAdminContext())
		})
		beego.BeeApp.Handlers.ServeHTTP(w, r)
		assertTestResult(t, w.Code, 500)
	})
}

func TestCreateFileShare(t *testing.T) {
	var jsonStr = []byte(`{
		"id": "d2975ebe-d82c-430f-b28e-f373746a71ca",
		"name":"sample-fileshare-01",
		"description":"This is first sample fileshare for testing",
		"size":1,
		"poolId": "a5965ebe-dg2c-434t-b28e-f373746a71ca",
		"snapshotId": "b7602e18-771e-11e7-8f38-dbd6d291f4eg"
	}`)

	t.Run("Should return 202 if everything works well", func(t *testing.T) {
		fileshare := model.FileShareSpec{BaseModel: &model.BaseModel{}}
		json.NewDecoder(bytes.NewBuffer(jsonStr)).Decode(&fileshare)
		fileshare.CreatedAt = time.Now().Format(constants.TimeFormat)
		fileshare.UpdatedAt = time.Now().Format(constants.TimeFormat)
		fileshare.AvailabilityZone = "default"
		fileshare.Status = "creating"
		fileshare.ProfileId = "1106b972-66ef-11e7-b172-db03f3689c9c"
		mockClient := new(dbtest.Client)
		mockClient.On("GetDefaultProfileFileShare", c.NewAdminContext()).Return(&SampleFileShareProfiles[0], nil)
		mockClient.On("GetFileShareSnapshot", c.NewAdminContext(), "b7602e18-771e-11e7-8f38-dbd6d291f4eg").Return(&SampleFileShareSnapshots[0], nil)
		mockClient.On("GetFileShare", c.NewAdminContext(), "d2975ebe-d82c-430f-b28e-f373746a71ca").Return(&SampleFileShares[0], nil)
		mockClient.On("CreateFileShare", c.NewAdminContext(), &fileshare).Return(&SampleFileShares[0], nil)
		db.C = mockClient

		r, _ := http.NewRequest("POST", "/v1beta/file/shares", bytes.NewBuffer(jsonStr))
		w := httptest.NewRecorder()
		r.Header.Set("Content-Type", "application/JSON")
		beego.InsertFilter("*", beego.BeforeExec, func(httpCtx *context.Context) {
			httpCtx.Input.SetData("context", c.NewAdminContext())
		})
		beego.BeeApp.Handlers.ServeHTTP(w, r)
		var output model.FileShareSpec
		json.Unmarshal(w.Body.Bytes(), &output)
		assertTestResult(t, w.Code, 202)
		assertTestResult(t, &output, &SampleFileShares[0])
	})

	t.Run("Should return 404 if create file share with bad request - Provided snapshot id not found in db", func(t *testing.T) {
		fileshare := model.FileShareSpec{BaseModel: &model.BaseModel{}}
		json.NewDecoder(bytes.NewBuffer(jsonStr)).Decode(&fileshare)
		mockClient := new(dbtest.Client)
		mockClient.On("GetDefaultProfileFileShare", c.NewAdminContext()).Return(&SampleFileShareProfiles[0], nil)
		mockClient.On("GetFileShareSnapshot", c.NewAdminContext(), "b7602e18-771e-11e7-8f38-dbd6d291f4eg").Return(nil, errors.New("specified fileshare snapshot(b7602e18-771e-11e7-8f38-dbd6d291f4eg) can't find"))
		db.C = mockClient

		r, _ := http.NewRequest("POST", "/v1beta/file/shares", bytes.NewBuffer(jsonStr))
		w := httptest.NewRecorder()
		r.Header.Set("Content-Type", "application/JSON")
		beego.InsertFilter("*", beego.BeforeExec, func(httpCtx *context.Context) {
			httpCtx.Input.SetData("context", c.NewAdminContext())
		})
		beego.BeeApp.Handlers.ServeHTTP(w, r)
		assertTestResult(t, w.Code, 404)
	})
}

func TestDeleteFileShare(t *testing.T) {

	t.Run("Should return 202 if delete file share is ok", func(t *testing.T) {
		mockClient := new(dbtest.Client)
		mockClient.On("GetFileShare", c.NewAdminContext(), "d2975ebe-d82c-430f-b28e-f373746a71ca").Return(&SampleFileShares[0], nil)
		mockClient.On("GetProfile", c.NewAdminContext(), "b3585ebe-c42c-120g-b28e-f373746a71ca").Return(&SampleFileShareProfiles[0], nil)
		mockClient.On("ListSnapshotsByShareId", c.NewAdminContext(), "d2975ebe-d82c-430f-b28e-f373746a71ca").Return(nil, nil)
		mockClient.On("ListFileShareAclsByShareId", c.NewAdminContext(), "d2975ebe-d82c-430f-b28e-f373746a71ca").Return(nil, nil)
		mockClient.On("UpdateFileShare", c.NewAdminContext(), &SampleFileShares[0]).Return(nil, nil)
		mockClient.On("DeleteFileShare", c.NewAdminContext(), "d2975ebe-d82c-430f-b28e-f373746a71ca").Return(nil)
		db.C = mockClient

		r, _ := http.NewRequest("DELETE", "/v1beta/file/shares/d2975ebe-d82c-430f-b28e-f373746a71ca", nil)
		w := httptest.NewRecorder()
		beego.InsertFilter("*", beego.BeforeExec, func(httpCtx *context.Context) {
			httpCtx.Input.SetData("context", c.NewAdminContext())
		})
		beego.BeeApp.Handlers.ServeHTTP(w, r)
		assertTestResult(t, w.Code, 202)
	})

	t.Run("Should return 404 if delete file share with bad request - file share id not found", func(t *testing.T) {
		mockClient := new(dbtest.Client)
		mockClient.On("GetFileShare", c.NewAdminContext(), "d2975ebe-d82c-430f-b28e-f373746a71ca").Return(nil, errors.New("specified fileshare(d2975ebe-d82c-430f-b28e-f373746a71ca) can't find"))
		db.C = mockClient

		r, _ := http.NewRequest("DELETE",
			"/v1beta/file/shares/d2975ebe-d82c-430f-b28e-f373746a71ca", nil)
		w := httptest.NewRecorder()
		beego.InsertFilter("*", beego.BeforeExec, func(httpCtx *context.Context) {
			httpCtx.Input.SetData("context", c.NewAdminContext())
		})
		beego.BeeApp.Handlers.ServeHTTP(w, r)
		assertTestResult(t, w.Code, 404)
	})
}

////////////////////////////////////////////////////////////////////////////////
//                      Tests for fileshare snapshot                          //
////////////////////////////////////////////////////////////////////////////////

func TestListFileShareSnapshots(t *testing.T) {

	t.Run("Should return 200 if everything works well", func(t *testing.T) {
		var sampleSnapshots = []*model.FileShareSnapshotSpec{&SampleFileShareSnapshots[0], &SampleFileShareSnapshots[1]}
		mockClient := new(dbtest.Client)
		m := map[string][]string{
			"offset":  {"0"},
			"limit":   {"1"},
			"sortDir": {"asc"},
			"sortKey": {"name"},
		}
		mockClient.On("ListFileShareSnapshotsWithFilter", c.NewAdminContext(), m).Return(sampleSnapshots, nil)
		db.C = mockClient

		r, _ := http.NewRequest("GET", "/v1beta/file/snapshots?offset=0&limit=1&sortDir=asc&sortKey=name", nil)
		w := httptest.NewRecorder()
		beego.InsertFilter("*", beego.BeforeExec, func(httpCtx *context.Context) {
			httpCtx.Input.SetData("context", c.NewAdminContext())
		})
		beego.BeeApp.Handlers.ServeHTTP(w, r)

		var output []*model.FileShareSnapshotSpec
		json.Unmarshal(w.Body.Bytes(), &output)
		assertTestResult(t, w.Code, 200)
		assertTestResult(t, output, sampleSnapshots)
	})

	t.Run("Should return 500 if list fileshare snapshots with bad request", func(t *testing.T) {
		mockClient := new(dbtest.Client)
		m := map[string][]string{
			"offset":  {"0"},
			"limit":   {"1"},
			"sortDir": {"asc"},
			"sortKey": {"name"},
		}
		mockClient.On("ListFileShareSnapshotsWithFilter", c.NewAdminContext(), m).Return(nil, errors.New("db error"))
		db.C = mockClient

		r, _ := http.NewRequest("GET", "/v1beta/file/snapshots?offset=0&limit=1&sortDir=asc&sortKey=name", nil)
		w := httptest.NewRecorder()
		beego.InsertFilter("*", beego.BeforeExec, func(httpCtx *context.Context) {
			httpCtx.Input.SetData("context", c.NewAdminContext())
		})
		beego.BeeApp.Handlers.ServeHTTP(w, r)
		assertTestResult(t, w.Code, 500)
	})
}

func TestGetFileShareSnapshot(t *testing.T) {

	t.Run("Should return 200 if everything works well", func(t *testing.T) {
		mockClient := new(dbtest.Client)
		mockClient.On("GetFileShareSnapshot", c.NewAdminContext(), "3769855c-a102-11e7-b772-17b880d2f537").Return(&SampleFileShareSnapshots[0], nil)
		db.C = mockClient

		r, _ := http.NewRequest("GET", "/v1beta/file/snapshots/3769855c-a102-11e7-b772-17b880d2f537", nil)
		w := httptest.NewRecorder()
		beego.InsertFilter("*", beego.BeforeExec, func(httpCtx *context.Context) {
			httpCtx.Input.SetData("context", c.NewAdminContext())
		})
		beego.BeeApp.Handlers.ServeHTTP(w, r)
		var output model.FileShareSnapshotSpec
		json.Unmarshal(w.Body.Bytes(), &output)
		assertTestResult(t, w.Code, 200)
		assertTestResult(t, &output, &SampleFileShareSnapshots[0])
	})

	t.Run("Should return 404 if get fileshare group with bad request", func(t *testing.T) {
		mockClient := new(dbtest.Client)
		mockClient.On("GetFileShareSnapshot", c.NewAdminContext(), "3769855c-a102-11e7-b772-17b880d2f537").Return(nil, errors.New("db error"))
		db.C = mockClient

		r, _ := http.NewRequest("GET", "/v1beta/file/snapshots/3769855c-a102-11e7-b772-17b880d2f537", nil)
		w := httptest.NewRecorder()
		beego.InsertFilter("*", beego.BeforeExec, func(httpCtx *context.Context) {
			httpCtx.Input.SetData("context", c.NewAdminContext())
		})
		beego.BeeApp.Handlers.ServeHTTP(w, r)
		assertTestResult(t, w.Code, 404)
	})
}

func TestUpdateFileShareSnapshot(t *testing.T) {
	var jsonStr = []byte(`{
		"id": "3769855c-a102-11e7-b772-17b880d2f537",
		"name":"fake snapshot",
		"description":"fake snapshot"
	}`)
	var expectedJson = []byte(`{
		"id": "3769855c-a102-11e7-b772-17b880d2f537",
		"name": "fake snapshot",
		"description": "fake snapshot",
		"size": 1,
		"status": "available",
		"fileshareId": "bd5b12a8-a101-11e7-941e-d77981b584d8"
	}`)
	var expected model.FileShareSnapshotSpec
	json.Unmarshal(expectedJson, &expected)

	t.Run("Should return 200 if everything works well", func(t *testing.T) {
		snapshot := model.FileShareSnapshotSpec{BaseModel: &model.BaseModel{}}
		json.NewDecoder(bytes.NewBuffer(jsonStr)).Decode(&snapshot)
		mockClient := new(dbtest.Client)
		mockClient.On("UpdateFileShareSnapshot", c.NewAdminContext(), snapshot.Id, &snapshot).
			Return(&expected, nil)
		db.C = mockClient

		r, _ := http.NewRequest("PUT", "/v1beta/file/snapshots/3769855c-a102-11e7-b772-17b880d2f537", bytes.NewBuffer(jsonStr))
		w := httptest.NewRecorder()
		r.Header.Set("Content-Type", "application/JSON")
		beego.InsertFilter("*", beego.BeforeExec, func(httpCtx *context.Context) {
			httpCtx.Input.SetData("context", c.NewAdminContext())
		})
		beego.BeeApp.Handlers.ServeHTTP(w, r)
		var output model.FileShareSnapshotSpec
		json.Unmarshal(w.Body.Bytes(), &output)
		assertTestResult(t, w.Code, 200)
		assertTestResult(t, &output, &expected)
	})

	t.Run("Should return 500 if update fileshare snapshot with bad request", func(t *testing.T) {
		snapshot := model.FileShareSnapshotSpec{BaseModel: &model.BaseModel{}}
		json.NewDecoder(bytes.NewBuffer(jsonStr)).Decode(&snapshot)
		mockClient := new(dbtest.Client)
		mockClient.On("UpdateFileShareSnapshot", c.NewAdminContext(), snapshot.Id, &snapshot).
			Return(nil, errors.New("db error"))
		db.C = mockClient

		r, _ := http.NewRequest("PUT", "/v1beta/file/snapshots/3769855c-a102-11e7-b772-17b880d2f537", bytes.NewBuffer(jsonStr))
		w := httptest.NewRecorder()
		r.Header.Set("Content-Type", "application/JSON")
		beego.InsertFilter("*", beego.BeforeExec, func(httpCtx *context.Context) {
			httpCtx.Input.SetData("context", c.NewAdminContext())
		})
		beego.BeeApp.Handlers.ServeHTTP(w, r)
		assertTestResult(t, w.Code, 500)
	})
}

func TestCreateFileShareSnapshot(t *testing.T) {
	var jsonStr = []byte(`{
		"id": "3769855c-a102-11e7-b772-17b880d2f537",
		"fileshareId": "d2975ebe-d82c-430f-b28e-f373746a71ca",
		"name": "File_share_snapshot",
		"description": "fake File share snapshot",
		"profileId": "1106b972-66ef-11e7-b172-db03f3689c9c",
		"shareSize": 1,
		"snapshotSize": 1
	}`)

	t.Run("Should return 202 if everything works well", func(t *testing.T) {
		snapshot := model.FileShareSnapshotSpec{BaseModel: &model.BaseModel{}}
		json.NewDecoder(bytes.NewBuffer(jsonStr)).Decode(&snapshot)
		snapshot.Status = "creating"
		snapshot.CreatedAt = time.Now().Format(constants.TimeFormat)

		mockClient := new(dbtest.Client)
		mockClient.On("GetFileShare", c.NewAdminContext(), SampleFileShareSnapshots[0].FileShareId).Return(&SampleFileShares[1], nil)
		mockClient.On("GetProfile", c.NewAdminContext(), "1106b972-66ef-11e7-b172-db03f3689c9c").Return(&SampleFileShareProfiles[0], nil)
		mockClient.On("ListFileShareSnapshots", c.NewAdminContext()).Return(nil, nil)
		mockClient.On("CreateFileShareSnapshot", c.NewAdminContext(), &snapshot).Return(&SampleFileShareSnapshots[0], nil)
		db.C = mockClient

		r, _ := http.NewRequest("POST", "/v1beta/file/snapshots", bytes.NewBuffer(jsonStr))
		w := httptest.NewRecorder()
		r.Header.Set("Content-Type", "application/JSON")
		beego.InsertFilter("*", beego.BeforeExec, func(httpCtx *context.Context) {
			httpCtx.Input.SetData("context", c.NewAdminContext())
		})
		beego.BeeApp.Handlers.ServeHTTP(w, r)
		var output model.FileShareSnapshotSpec
		json.Unmarshal(w.Body.Bytes(), &output)
		assertTestResult(t, w.Code, 202)
		assertTestResult(t, &output, &SampleFileShareSnapshots[0])
	})

	t.Run("Should return 404 if create file share snapshot with bad request - fileshare id which doesn't exists in db", func(t *testing.T) {
		mockClient := new(dbtest.Client)
		mockClient.On("GetFileShare", c.NewAdminContext(), SampleFileShareSnapshots[0].FileShareId).Return(nil, errors.New("specified fileshare (b7602e18-771e-11e7-8f38-dbd6d291f4eg) can't find"))
		db.C = mockClient

		r, _ := http.NewRequest("POST", "/v1beta/file/snapshots", bytes.NewBuffer(jsonStr))
		w := httptest.NewRecorder()
		r.Header.Set("Content-Type", "application/JSON")
		beego.InsertFilter("*", beego.BeforeExec, func(httpCtx *context.Context) {
			httpCtx.Input.SetData("context", c.NewAdminContext())
		})
		beego.BeeApp.Handlers.ServeHTTP(w, r)
		assertTestResult(t, w.Code, 404)
	})
}

func TestDeleteFileShareSnapshot(t *testing.T) {

	t.Run("Should return 202 if everything works well", func(t *testing.T) {
		mockClient := new(dbtest.Client)
		mockClient.On("GetFileShareSnapshot", c.NewAdminContext(), "3769855c-a102-11e7-b772-17b880d2f537").Return(&SampleFileShareSnapshots[0], nil)
		mockClient.On("GetProfile", c.NewAdminContext(), SampleFileShareSnapshots[0].ProfileId).Return(&SampleFileShareProfiles[0], nil)
		mockClient.On("GetFileShare", c.NewAdminContext(), SampleFileShareSnapshots[0].FileShareId).Return(&SampleFileShares[0], nil)
		mockClient.On("UpdateFileShareSnapshot", c.NewAdminContext(), SampleFileShareSnapshots[0].Id, &SampleFileShareSnapshots[0]).Return(nil, nil)
		mockClient.On("DeleteFileShareSnapshot", c.NewAdminContext(), "3769855c-a102-11e7-b772-17b880d2f537").Return(nil)
		db.C = mockClient

		r, _ := http.NewRequest("DELETE",
			"/v1beta/file/snapshots/3769855c-a102-11e7-b772-17b880d2f537", nil)
		w := httptest.NewRecorder()
		beego.InsertFilter("*", beego.BeforeExec, func(httpCtx *context.Context) {
			httpCtx.Input.SetData("context", c.NewAdminContext())
		})
		beego.BeeApp.Handlers.ServeHTTP(w, r)
		assertTestResult(t, w.Code, 202)
	})

	t.Run("Should return 404 if delete file share snapshot with bad request - snapshot id doesn't exist in db ", func(t *testing.T) {
		mockClient := new(dbtest.Client)
		mockClient.On("GetFileShareSnapshot", c.NewAdminContext(), "3769855c-a102-11e7-b772-17b880d2f537").Return(nil, errors.New("specified fileshare snapshot(b7602e18-771e-11e7-8f38-dbd6d291f4eg) can't find"))
		db.C = mockClient

		r, _ := http.NewRequest("DELETE",
			"/v1beta/file/snapshots/3769855c-a102-11e7-b772-17b880d2f537", nil)
		w := httptest.NewRecorder()
		beego.InsertFilter("*", beego.BeforeExec, func(httpCtx *context.Context) {
			httpCtx.Input.SetData("context", c.NewAdminContext())
		})
		beego.BeeApp.Handlers.ServeHTTP(w, r)
		assertTestResult(t, w.Code, 404)
	})
}

////////////////////////////////////////////////////////////////////////////////
//                      Tests for fileshare ACL                               //
////////////////////////////////////////////////////////////////////////////////

func TestCreateFileShareAcl(t *testing.T) {
	var jsonStr = []byte(`{
		"id": "6ad25d59-a160-45b2-8920-211be282e2df",
		"fileshareId": "d2975ebe-d82c-430f-b28e-f373746a71ca",
		"type": "ip",
		"accessCapability": [
			"Read", "Write"
		],
		"accessTo": "10.32.109.15",
		"profileId": "1106b972-66ef-11e7-b172-db03f3689c9c",
		"description": "This is a sample Acl for testing"
	}`)

	t.Run("Should return 202 if everything works well", func(t *testing.T) {
		acl := model.FileShareAclSpec{BaseModel: &model.BaseModel{}}
		json.NewDecoder(bytes.NewBuffer(jsonStr)).Decode(&acl)
		acl.CreatedAt = time.Now().Format(constants.TimeFormat)
		acl.UpdatedAt = time.Now().Format(constants.TimeFormat)
		acl.Status = "available"
		SampleFileShares[0].Status = "available"
		acl.Metadata = map[string]string(nil)
		mockClient := new(dbtest.Client)
		mockClient.On("GetFileShare", c.NewAdminContext(), SampleFileSharesAcl[2].FileShareId).Return(&SampleFileShares[0], nil)
		mockClient.On("GetProfile", c.NewAdminContext(), "1106b972-66ef-11e7-b172-db03f3689c9c").Return(&SampleFileShareProfiles[0], nil)
		mockClient.On("CreateFileShareAcl", c.NewAdminContext(), &acl).Return(&SampleFileSharesAcl[2], nil)
		db.C = mockClient

		r, _ := http.NewRequest("POST", "/v1beta/file/acls", bytes.NewBuffer(jsonStr))
		w := httptest.NewRecorder()
		r.Header.Set("Content-Type", "application/JSON")
		beego.InsertFilter("*", beego.BeforeExec, func(httpCtx *context.Context) {
			httpCtx.Input.SetData("context", c.NewAdminContext())
		})
		beego.BeeApp.Handlers.ServeHTTP(w, r)
		var output model.FileShareAclSpec
		json.Unmarshal(w.Body.Bytes(), &output)
		assertTestResult(t, w.Code, 202)
		assertTestResult(t, &output, &SampleFileSharesAcl[2])
	})

	t.Run("Should return 400 if create file share acl with bad request - fileshare acl id which doesn't exists in db", func(t *testing.T) {
		acl := model.FileShareAclSpec{BaseModel: &model.BaseModel{}}
		json.NewDecoder(bytes.NewBuffer(jsonStr)).Decode(&acl)
		acl.CreatedAt = time.Now().Format(constants.TimeFormat)
		acl.UpdatedAt = time.Now().Format(constants.TimeFormat)
		acl.Status = "available"
		mockClient := new(dbtest.Client)
		mockClient.On("GetFileShare", c.NewAdminContext(), SampleFileSharesAcl[2].FileShareId).Return(nil, errors.New("specified fileshare (b7602e18-771e-11e7-8f38-dbd6d291f4eg) can't find"))
		db.C = mockClient

		r, _ := http.NewRequest("POST", "/v1beta/file/acls", bytes.NewBuffer(jsonStr))
		w := httptest.NewRecorder()
		r.Header.Set("Content-Type", "application/JSON")
		beego.InsertFilter("*", beego.BeforeExec, func(httpCtx *context.Context) {
			httpCtx.Input.SetData("context", c.NewAdminContext())
		})
		beego.BeeApp.Handlers.ServeHTTP(w, r)
		assertTestResult(t, w.Code, 400)
	})
}

func TestListFileSharesAcl(t *testing.T) {
	t.Run("Should return 200 if everything works well", func(t *testing.T) {
		var sampleAcls = []*model.FileShareAclSpec{&SampleFileSharesAcl[0], &SampleFileSharesAcl[1]}
		mockClient := new(dbtest.Client)
		m := map[string][]string{
			"offset":  {"0"},
			"limit":   {"1"},
			"sortDir": {"asc"},
			"sortKey": {"name"},
		}
		mockClient.On("ListFileSharesAclWithFilter", c.NewAdminContext(), m).Return(sampleAcls, nil)
		db.C = mockClient

		r, _ := http.NewRequest("GET", "/v1beta/file/acls?offset=0&limit=1&sortDir=asc&sortKey=name", nil)
		w := httptest.NewRecorder()
		beego.InsertFilter("*", beego.BeforeExec, func(httpCtx *context.Context) {
			httpCtx.Input.SetData("context", c.NewAdminContext())
		})
		beego.BeeApp.Handlers.ServeHTTP(w, r)

		var output []*model.FileShareAclSpec
		json.Unmarshal(w.Body.Bytes(), &output)
		assertTestResult(t, w.Code, 200)
		assertTestResult(t, output, sampleAcls)
	})

	t.Run("Should return 500 if list fileshare acl with bad request - db error to list acls", func(t *testing.T) {
		mockClient := new(dbtest.Client)
		m := map[string][]string{
			"offset":  {"0"},
			"limit":   {"1"},
			"sortDir": {"asc"},
			"sortKey": {"name"},
		}
		mockClient.On("ListFileSharesAclWithFilter", c.NewAdminContext(), m).Return(nil, errors.New("db error"))
		db.C = mockClient

		r, _ := http.NewRequest("GET", "/v1beta/file/acls?offset=0&limit=1&sortDir=asc&sortKey=name", nil)
		w := httptest.NewRecorder()
		beego.InsertFilter("*", beego.BeforeExec, func(httpCtx *context.Context) {
			httpCtx.Input.SetData("context", c.NewAdminContext())
		})
		beego.BeeApp.Handlers.ServeHTTP(w, r)
		assertTestResult(t, w.Code, 500)
	})
}

func TestGetFileShareAcl(t *testing.T) {
	t.Run("Should return 200 if everything works well", func(t *testing.T) {
		mockClient := new(dbtest.Client)
		mockClient.On("GetFileShareAcl", c.NewAdminContext(), "6ad25d59-a160-45b2-8920-211be282e2df").Return(&SampleFileSharesAcl[2], nil)
		db.C = mockClient

		r, _ := http.NewRequest("GET", "/v1beta/file/acls/6ad25d59-a160-45b2-8920-211be282e2df", nil)
		w := httptest.NewRecorder()
		beego.InsertFilter("*", beego.BeforeExec, func(httpCtx *context.Context) {
			httpCtx.Input.SetData("context", c.NewAdminContext())
		})
		beego.BeeApp.Handlers.ServeHTTP(w, r)
		var output model.FileShareAclSpec
		json.Unmarshal(w.Body.Bytes(), &output)
		assertTestResult(t, w.Code, 200)
		assertTestResult(t, &output, &SampleFileSharesAcl[2])
	})

	t.Run("Should return 404 if get fileshare acl with bad request - acl id not found in db", func(t *testing.T) {
		mockClient := new(dbtest.Client)
		mockClient.On("GetFileShareAcl", c.NewAdminContext(), "6ad25d59-a160-45b2-8920-211be282e2df").Return(nil, errors.New("specified fileshare snapshot(6ad25d59-a160-45b2-8920-211be282e2df) can't find"))
		db.C = mockClient

		r, _ := http.NewRequest("GET", "/v1beta/file/acls/6ad25d59-a160-45b2-8920-211be282e2df", nil)
		w := httptest.NewRecorder()
		beego.InsertFilter("*", beego.BeforeExec, func(httpCtx *context.Context) {
			httpCtx.Input.SetData("context", c.NewAdminContext())
		})
		beego.BeeApp.Handlers.ServeHTTP(w, r)
		assertTestResult(t, w.Code, 404)
	})
}

func TestDeleteFileShareAcl(t *testing.T) {
	t.Run("Should return 202 if everything works well", func(t *testing.T) {
		SampleFileSharesAcl[2].Status = "available"
		mockClient := new(dbtest.Client)
		mockClient.On("GetFileShareAcl", c.NewAdminContext(), "6ad25d59-a160-45b2-8920-211be282e2df").Return(&SampleFileSharesAcl[2], nil)
		mockClient.On("GetProfile", c.NewAdminContext(), "b3585ebe-c42c-120g-b28e-f373746a71ca").Return(&SampleFileShareProfiles[0], nil)
		mockClient.On("GetFileShare", c.NewAdminContext(), SampleFileSharesAcl[2].FileShareId).Return(&SampleFileShares[0], nil)
		mockClient.On("UpdateFileShareAcl", c.NewAdminContext(), &SampleFileSharesAcl[2]).Return(nil, nil)
		mockClient.On("DeleteFileShareAcl", c.NewAdminContext(), "6ad25d59-a160-45b2-8920-211be282e2df").Return(nil)
		db.C = mockClient

		r, _ := http.NewRequest("DELETE",
			"/v1beta/file/acls/6ad25d59-a160-45b2-8920-211be282e2df", nil)
		w := httptest.NewRecorder()
		beego.InsertFilter("*", beego.BeforeExec, func(httpCtx *context.Context) {
			httpCtx.Input.SetData("context", c.NewAdminContext())
		})
		beego.BeeApp.Handlers.ServeHTTP(w, r)
		assertTestResult(t, w.Code, 202)
	})

	t.Run("Should return 404 if delete file share acl with bad request - invalid file share acl id", func(t *testing.T) {
		mockClient := new(dbtest.Client)
		mockClient.On("GetFileShareAcl", c.NewAdminContext(), "6ad25d59-a160-45b2-8920-211be282e2df").Return(nil, errors.New("specified fileshare acl(6ad25d59-a160-45b2-8920-211be282e2df) can't find"))
		db.C = mockClient

		r, _ := http.NewRequest("DELETE",
			"/v1beta/file/acls/6ad25d59-a160-45b2-8920-211be282e2df", nil)
		w := httptest.NewRecorder()
		beego.InsertFilter("*", beego.BeforeExec, func(httpCtx *context.Context) {
			httpCtx.Input.SetData("context", c.NewAdminContext())
		})
		beego.BeeApp.Handlers.ServeHTTP(w, r)
		assertTestResult(t, w.Code, 404)
	})
}
