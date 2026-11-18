/*
Copyright 2026 DataKnifeAI.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package config

import "testing"

func TestParseWorldSaveGuid(t *testing.T) {
	log := `[2026.11.16-05.15.13:106]LogPersistence: [DedicatedServer] SaveGameInternal() : Starting save (Guid[49EBD94A739B4F21BE86570EA7219A23] WorldName[Factory Test] SlotName[Factory Test]`
	if got := ParseWorldSaveGuid(log); got != "49EBD94A739B4F21BE86570EA7219A23" {
		t.Fatalf("got %q", got)
	}
}

func TestParseBuildID(t *testing.T) {
	log := `LogNetSessionSettings: Setting ["BuildId"] written with key[xb] value[258140]`
	if got := ParseBuildID(log); got != "258140" {
		t.Fatalf("got %q", got)
	}
}

func TestHasSuccessfulJoin(t *testing.T) {
	if !HasSuccessfulJoin("LogNet: Join succeeded: Surreal Wolf") {
		t.Fatal("expected join")
	}
}
