// Copyright 2021 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// [START redis_v1_generated_CloudRedis_DeleteInstance_sync]

package main

import (
	"context"

	redis "cloud.google.com/go/redis/apiv1"
	redispb "google.golang.org/genproto/googleapis/cloud/redis/v1"
)

func main() {
	// import redispb "google.golang.org/genproto/googleapis/cloud/redis/v1"

	ctx := context.Background()
	c, err := redis.NewCloudRedisClient(ctx)
	if err != nil {
		// TODO: Handle error.
	}

	req := &redispb.DeleteInstanceRequest{
		// TODO: Fill request struct fields.
	}
	op, err := c.DeleteInstance(ctx, req)
	if err != nil {
		// TODO: Handle error.
	}

	err = op.Wait(ctx)
	if err != nil {
		// TODO: Handle error.
	}
}

// [END redis_v1_generated_CloudRedis_DeleteInstance_sync]