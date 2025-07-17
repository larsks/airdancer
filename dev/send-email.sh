#!/bin/sh

exec swaks --to dancer --from testing --server localhost:31024 --protocol LMTP --body "$*"
