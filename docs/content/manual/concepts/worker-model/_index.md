+++
title = "Worker Model"
weight = 9

+++

### Purpose

The goal of CDS is to start a worker when you need it, which should match all your requirements exactly.
In order to scale automatically on demand, it is possible to register a worker model.

The goal of a worker model is to describe the capabilities of a given Docker/ISO image in terms of architecture, pre-installed binaries or libraries.

### Types

There are 4 types of worker models:

 * Docker images, see [how to create a worker model Docker]({{< relref "/manual/gettingstarted/tutorials/worker_model-docker/_index.md" >}})
 * OpenStack images, see [how to create a worker model OpenStack]({{< relref "/manual/gettingstarted/tutorials/worker_model-openstack.md" >}})
 * vSphere images, see [how to create a worker model vSphere]({{< relref "/manual/gettingstarted/tutorials/worker_model-vsphere.md" >}})
 * Host worker models, which is a worker launched on the same host than the hatchery, we don't recommend to use this in production.

### Behavior

All registered CDS [hatcheries]({{< relref "/manual/components/hatchery/_index.md" >}}) start/kill workers as needed.

### Add a worker model

![Add a worker model](/images/workflows.pipelines.requirements.docker.worker-model.add.png)

A user can add a worker model by setting an owner group if user is administrator of group.

A CDS administrator can add a worker model, a [worker model pattern]({{< relref "/manual/concepts/worker-model/patterns.md" >}}), attach it to 'shared.infra' group and set provision as he want.

### What's a worker model pattern?

A pattern of scripts configuration of CDS workers that CDS administrator create to let users use this pattern when they want to create a worker model. For more information [click here]({{< relref "/manual/concepts/worker-model/patterns.md" >}}).

### What's a restricted worker model?

A `shared.infra` hatchery can launch all worker models, except 'restricted' worker models.

**Use case**: users can launch their own [hatchery]({{< relref "/manual/components/hatchery/_index.md" >}}).
To use their worker models only with their hatchery, they have to set worker model as 'restricted'.

### What's workers provisioning?

A [hatchery]({{< relref "/manual/components/hatchery/_index.md" >}}) can start workers based on worker models with provisioning > 0.

On 'restricted' worker models, users can set provisioning, as they launch CDS Workers on their infrastructure.

Otherwise, provisioning is only editable by CDS Administrators.

**Notice**: if you use [Service Requirement]({{< relref "/manual/concepts/requirement/requirement_service.md" >}}), you can't
use provisioned workers.