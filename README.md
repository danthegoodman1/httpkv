# httpkv

## Go and Rust?

Yes. Partly because I couldn't decide between wanting to use this as a Rust learning project and wanting to make it quickly with familiar tools.

## Why

Data storage solutions are getting more and more complicated.

Their building for complex usage cases like vector-search, using the GPU for accelerated aggregation, and more. They’re solving really big problems in their own dimensions.

Yet everyone’s favorite database is dead-simple Redis (at least it used to be).

The problem with really fast databases is they tend to require a really fast protocol for them, and rightfully so: It’d be pretty lame if your super fast database double its response time because of JSON (de)serialization over HTTP.

But sometimes that’s ok. But sometimes it’s quite unnecessary.

Sometimes 15ms or 32ms is just as acceptable as 3ms for reads.

Sometimes you’re using a language (bash) or runtime (v8 isolate) that can’t use that fancy TCP or UDP-based client with custom serialization formats.

Sometimes you just want to store whether you’ve already done something or not, we don’t need a typed-schema or JSON.

Sometimes you just want to be able to make a list of things, and be able to scan over it in order (and reverse order as well… looking at you S3).

**Sometimes, we just need a KV that you can hit like a monster.**

Imagine a database where you can just `curl mydb.httpkv.com/somekey -d '1'` to write data and later get it with `curl mydb.httpkv.com/somekey`

George Hotz did this with [minikeyvalue](https://github.com/geohot/minikeyvalue) at comma, but you don’t want to spin that infra up yourself.

So why make this then?

You need a database now. Like in 30 seconds or less that you can hit at 1,000 req/s. I got you.

This is managed, which means it’s ready when you are. It can scale to what you need (need a few billion values stored? I got you).

The billing is simple: Egress and storage. Note that you’re not billed for the number of operations, or which operation you do (and you aren’t billed for unauthenticated requests or 5xx responses either).

There’s a baby free tier so you can test it out, but enough to where it won’t be drained by anti-capitalists.

Is this just a convenient wrapper around a database? No, it’s a wrapper + usage metering + scaling :)

But the point is even similar HTTP-based solutions like Firestore and DynamoDB require some amount of set up, and you really need client packages to use them (also they’re hash-partitioned, not range-partitioned, and range-partitioning is nicer to use).

It also doesn’t have insane egress fees, so you don’t have to worry about getting skinned when running outside of GCP/AWS (or even the same region).

**This is exactly that: A dead-simple KV database over HTTP.**

Your client: anything.

- curl
- wget
- `const res = await fetch`
- `from requests import get, post, delete`
- A web browser

This is the return to monke of databases.

But it’s not too primitive, you can still do things like listing over keys (asc or desc), create if not exists, update if exists, atomic updates, etc.

# Design

## Just store bytes

Store JSON, protobuf, parquet, csv, glob, text, what ever you want. Serialize and deserialization is up to you, HTTPKV ignores the `content-type` header.

## Consistent operations

Operations are consistent: anything you complete is immediately seen by subsequent operations.

## Auth

You can authenticate your requests in many ways, which ever is easiest for how you’re making the request:

- The `auth` header (because `Authorization: Bearer {token}` is way too verbose)
- The `auth` query param
- Basic auth with any username (it’s ignored)

## Listing

When you list, by default the response is new-line separated keys.

`curl mydb.httpkv.com/prefix?list`

(omit the `/prefix` to list from the beginning)

This will return a default max limit of 1000 items, you can change that with the `limit=n` query param. You can set the offset with the `from=abc` (non-inclusive) param. You can reverse sort with the `reverse` param. If you reverse with no prefix it will start at the end. If you use the `from` query param while reverse sorting it will start at that point and work backwards from there.

This makes it dead simple to paginate forward and backward.

### Listing Values

You can choose to list keys and values together with the `vals` and `list` query params combined. This will return a new-line separated list in the format `{key}:{b64 value}`.

## Conditional Write

There are 2 kinds of conditional writes:

- Write if not exists
- Update if exists

By default, any `POST` request will upsert the value.

Getting fancy with HTTP methods if tricky and verbose, so we keep it simple with query params.

You can choose to write if the value does not exist with the `nx` query param, which will return a 409 if the value already exists.

You can update only if the value exists with the `ix` query param, which will return a 404 if the value does not already exist. You can also pass in a `version` key (available in the `version` response header) if you need to only update is it’s the previous version you saw, so concurrent updates won’t overwrite each other if you don’t want them to.

You can use the `version` key for atomic compare-and-swap.

If there are both provided, then we’ll mention it in the 400 response :)

## Status codes

Status codes can be generalized to their ranges for ease of use:

- 2xx: It did what you wanted
- 3xx: Redirect time!
- 4xx: You messed up
- 5xx: We messed up (or network error)

Any actually important information will be returned in the body for 4xx and 5xx codes, the particular status code will be associated with the respective issue type, but since those can be shared by multiple errors, the body of the request will always tell you exactly what was wrong (and generally how to fix it, because good error messages are often more useful than docs, especially when you’re trying to move fast).

## Regions

Databases exist in regions, but are available from anywhere.

We many introduce eventually consistent replication across regions if largely requested.

## Byte Range Reads

You can read a range of bytes in the key with the `start` and `end` query params, as integers. You will only be billed for the bytes that are returned, so if you know you only need the first X bytes (e.g. you’re storing parquet and need the metadata) then you can save money per-request.

## KV Sizes

The max size of a key is 1KB (in the URL), and the max size of a value is 95Ki (we save some space for metadata).