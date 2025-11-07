local project = import 'brewkit/project.libsonnet';

local appIDs = [
    'order',
];

local proto = [
    'api/client/testinternal/testinternal.proto',
    'api/server/orderinternal/orderinternal.proto',
];

project.project(appIDs, proto)