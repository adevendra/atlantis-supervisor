/* Copyright 2014 Ooyala, Inc. All rights reserved.
 *
 * This file is licensed under the Apache License, Version 2.0 (the "License"); you may not use this file
 * except in compliance with the License. You may obtain a copy of the License at
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software distributed under the License is
 * distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and limitations under the License.
 */

package rpc

import (
	. "atlantis/common"
	"atlantis/supervisor/containers"
	. "atlantis/supervisor/rpc/types"
	"errors"
	"fmt"
)

type AuthorizeSSHExecutor struct {
	arg   SupervisorAuthorizeSSHArg
	reply *SupervisorAuthorizeSSHReply
}

func (e *AuthorizeSSHExecutor) Request() interface{} {
	return e.arg
}

func (e *AuthorizeSSHExecutor) Result() interface{} {
	return e.reply
}

func (e *AuthorizeSSHExecutor) Description() string {
	return fmt.Sprintf("%s @ %s :\n%s", e.arg.User, e.arg.ContainerID, e.arg.PublicKey)
}

func (e *AuthorizeSSHExecutor) Authorize() error {
	return nil
}

func (e *AuthorizeSSHExecutor) Execute(t *Task) error {
	if e.arg.PublicKey == "" {
		return errors.New("Please specify an SSH public key.")
	}
	if e.arg.ContainerID == "" {
		return errors.New("Please specify a container id.")
	}
	if e.arg.User == "" {
		return errors.New("Please specify a user.")
	}
	cont := containers.Get(e.arg.ContainerID)
	if cont == nil {
		e.reply.Status = StatusError
		return errors.New("Unknown Container.")
	}
	if err := containers.AuthorizeSSHUser(cont, e.arg.User, e.arg.PublicKey); err != nil {
		e.reply.Status = StatusError
		return err
	}
	t.Log("[RPC][AuthorizeSSH] authorized %d", cont.SSHPort)
	e.reply.Port = cont.SSHPort
	e.reply.Status = StatusOk
	return nil
}

func (ih *Supervisor) AuthorizeSSH(arg SupervisorAuthorizeSSHArg, reply *SupervisorAuthorizeSSHReply) error {
	return NewTask("AuthorizeSSH", &AuthorizeSSHExecutor{arg, reply}).Run()
}

type DeauthorizeSSHExecutor struct {
	arg   SupervisorDeauthorizeSSHArg
	reply *SupervisorDeauthorizeSSHReply
}

func (e *DeauthorizeSSHExecutor) Request() interface{} {
	return e.arg
}

func (e *DeauthorizeSSHExecutor) Result() interface{} {
	return e.reply
}

func (e *DeauthorizeSSHExecutor) Description() string {
	return fmt.Sprintf("%s @ %s", e.arg.User, e.arg.ContainerID)
}

func (e *DeauthorizeSSHExecutor) Authorize() error {
	return nil
}

func (e *DeauthorizeSSHExecutor) Execute(t *Task) error {
	if e.arg.ContainerID == "" {
		return errors.New("Please specify a container id.")
	}
	if e.arg.User == "" {
		return errors.New("Please specify a user.")
	}
	cont := containers.Get(e.arg.ContainerID)
	if cont == nil {
		e.reply.Status = StatusError
		return errors.New("Unknown Container.")
	}
	if err := containers.DeauthorizeSSHUser(cont, e.arg.User); err != nil {
		e.reply.Status = StatusError
		return err
	}
	e.reply.Status = StatusOk
	return nil
}

func (ih *Supervisor) DeauthorizeSSH(arg SupervisorDeauthorizeSSHArg, reply *SupervisorDeauthorizeSSHReply) error {
	return NewTask("DeauthorizeSSH", &DeauthorizeSSHExecutor{arg, reply}).Run()
}
