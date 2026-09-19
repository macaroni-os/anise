/*
Copyright © 2021-2026 Daniele Rondina <geaaru@macaronios.org>

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program. If not, see <http://www.gnu.org/licenses/>.
*/
package specs

import (
	"encoding/json"

	"gopkg.in/yaml.v3"
)

func NewTaskSummary() *TaskSummary {
	return &TaskSummary{
		Files: []*FileIdentity{},
	}
}

func NewFileIdentity(t byte, name string) *FileIdentity {
	return &FileIdentity{
		Type: t,
		Name: name,
	}
}

func (s *TaskSummary) AddFile(f *FileIdentity) {
	s.Files = append(s.Files, f)
}

func (s *TaskSummary) GetFiles() []*FileIdentity { return s.Files }

func (s *TaskSummary) YAML() ([]byte, error) {
	return yaml.Marshal(s)
}

func (s *TaskSummary) ToJSON() ([]byte, error) {
	return json.Marshal(s)
}
