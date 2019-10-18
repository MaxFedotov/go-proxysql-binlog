#!/bin/bash
#
# Script to create new release on github
# Needs hub (https://github.com/github/hub) installed in order to publish release on github
# Increments current release version by 1
# Checks for release-notes file in docs\release-notes folder for new version and attaches them for release
# Bulds artifacts and attaches them for release

diff=$(git diff)
version=$(cat VERSION)
tag=v$version

if [ $? != 0 ]; then
    echo "Errors on git diff. Exiting"
    exit 0
fi

if [ $? "$diff" ]; then
    echo "Found pending changes. Commit them before creating release. Exiting"
    exit 0
fi

diff=$(git diff $tag)

if [ $? != 0 ]; then
    echo "Errors on git diff $tag. Exiting"
    exit 0
fi

if [ $? = 128 ]; then
    echo "Errors on git diff $tag. Unknown tag. Exiting"
    exit 0
fi

if [ -z "diff" ]; then
    echo "No changes from $version. Exiting"
    exit 0
fi

echo "Updating version"
echo "OLD"
echo "version: $version, tag: $tag"
new_version=$(echo "$version" | awk -F. '{$NF = $NF + 1;} 1' | sed 's/ /./g')
new_tag=v$new_version
echo "NEW"
echo "version: $new_version, tag: $new_tag"

echo "Checking for release notes for version $new_version"
if [ ! -f docs/release-notes/$new_version.md ]; then
    echo "Release notes file docs/release-notes/$new_version.md not found. Exiting"
    exit 0
fi

echo "Updating VERSION file"
echo "$new_version" > VERSION

echo "Creating packages for release"
rm -rf build/release/
mkdir build/release/
. build.sh -t linux -P

echo "Commiting changes"
git add VERSION docs/release-notes/$new_version.md
git commit -m "Creating release $new_version"

echo "Creating new tag $new_tag"
git tag $new_tag

echo "Pushing changes and tag"
git push
git push origin $new_tag

echo "Publishing release"
release_files=$(find build/release -name '*.deb' -o -name '*.rpm' -o -name '*.tar.gz' | while read line; do echo -a $line; done | paste -s -d' ' -)
hub release create -o -F docs/release-notes/$new_version.md $new_tag $release_files







