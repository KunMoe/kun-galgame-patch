-- 000_baseline.up.sql — Authoritative snapshot of the moyu (kungalgame_patch) schema.
--
-- WHAT THIS IS
--   pg_dump -s of the production-snapshot DB, then sanitized for replay
--   safety: every DDL is wrapped so re-running on a DB that already has the
--   tables / constraints is a no-op (logs NOTICE, never ERROR).
--
-- WHY IT EXISTS
--   moyu's schema was originally created by Prisma + the legacy Nitro
--   backend. The Go API never ran AutoMigrate; migrations 001-009 are pure
--   increments. That left the FK / index / sequence definitions as ghost
--   constraints — present in production (which restored from a Prisma-era
--   pg_dump backup), invisible to anyone trying to bootstrap a fresh DB from
--   the Go repo alone. See docs/proj/schema-ownership.md for the design.
--
-- WHAT IT GUARANTEES POST-EXECUTION
--   - 23 tables, 19 sequences, 19 indexes, 56 constraints (incl. 32 FKs:
--     28 CASCADE, 2 RESTRICT, 2 SET NULL).
--   - Identical to what `reset_all.sh` + `kungalgame_patch_backup.dump`
--     produces, captured 2026-05-26.
--
-- IDEMPOTENCY MECHANISM
--   - CREATE TABLE / SEQUENCE / INDEX use IF NOT EXISTS — on a populated DB
--     they short-circuit before column-comparison, so PRE-EXISTING TABLE
--     COLUMNS ARE NEVER MODIFIED. (Crucial: the legacy backup has columns
--     the Go API later drops in 005; baseline does not undo that.)
--   - ALTER TABLE ... ADD CONSTRAINT lives inside DO blocks; WHEN OTHERS
--     catches all 4xx-ish 23xxx / 42xxx SQLSTATEs.
--   - All sanitize steps preserve the original semantics — only failure
--     modes change.
--
-- ROLLOUT PATHS
--   Prisma-restore path (reset_all.sh + kungalgame_patch_backup.dump):
--     `cmd/migrate-oauth-prep` writes BOTH its own marker and
--     `000_baseline` into `_migrations` after applying. The subsequent
--     `cmd/migrate` therefore SKIPS this file entirely. This is REQUIRED:
--     on a Prisma-era schema, baseline's CREATE INDEX statements would
--     reference columns 002/004 haven't created yet (s3_key, galgame_id),
--     and CREATE INDEX IF NOT EXISTS doesn't pre-check column presence —
--     baseline would abort mid-file.
--
--   Fresh DB path (CI / DR / new env, no Prisma backup):
--     `cmd/migrate` runs baseline first (no oauth-prep step), then
--     001-009 layer their increments on top — all of those are idempotent
--     guarded so they no-op against the post-009 shape baseline produces.

--
-- PostgreSQL database dump
--

-- Dumped from database version 18.4
-- Dumped by pg_dump version 18.4

--
-- Name: chat_message_status; Type: TYPE; Schema: public; Owner: -
--

DO $$ BEGIN
  CREATE TYPE public.chat_message_status AS ENUM (
    'SENT',
    'EDITED',
    'DELETED'
);
EXCEPTION WHEN duplicate_object THEN
  RAISE NOTICE 'baseline skip: type public.chat_message_status already exists';
END $$;

--
-- Name: chat_role; Type: TYPE; Schema: public; Owner: -
--

DO $$ BEGIN
  CREATE TYPE public.chat_role AS ENUM (
    'OWNER',
    'ADMIN',
    'MEMBER'
);
EXCEPTION WHEN duplicate_object THEN
  RAISE NOTICE 'baseline skip: type public.chat_role already exists';
END $$;

--
-- Name: chat_type; Type: TYPE; Schema: public; Owner: -
--

DO $$ BEGIN
  CREATE TYPE public.chat_type AS ENUM (
    'PRIVATE',
    'GROUP'
);
EXCEPTION WHEN duplicate_object THEN
  RAISE NOTICE 'baseline skip: type public.chat_type already exists';
END $$;

--
-- Name: _migrations; Type: TABLE; Schema: public; Owner: -
--

--
-- Name: _migrations_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

--
-- Name: _migrations_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

--
-- Name: admin_log; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE IF NOT EXISTS public.admin_log (
    id integer NOT NULL,
    type text NOT NULL,
    content character varying(10007) NOT NULL,
    status integer DEFAULT 0 NOT NULL,
    user_id integer NOT NULL,
    created timestamp(3) without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated timestamp(3) without time zone NOT NULL
);

--
-- Name: admin_log_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE IF NOT EXISTS public.admin_log_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

--
-- Name: admin_log_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.admin_log_id_seq OWNED BY public.admin_log.id;

--
-- Name: chat_member; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE IF NOT EXISTS public.chat_member (
    id integer NOT NULL,
    role public.chat_role DEFAULT 'MEMBER'::public.chat_role NOT NULL,
    user_id integer NOT NULL,
    chat_room_id integer NOT NULL,
    created timestamp(3) without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated timestamp(3) without time zone NOT NULL
);

--
-- Name: chat_member_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE IF NOT EXISTS public.chat_member_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

--
-- Name: chat_member_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.chat_member_id_seq OWNED BY public.chat_member.id;

--
-- Name: chat_message; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE IF NOT EXISTS public.chat_message (
    id integer NOT NULL,
    content character varying(2000) DEFAULT ''::character varying NOT NULL,
    file_url character varying(1007) DEFAULT ''::character varying NOT NULL,
    status public.chat_message_status DEFAULT 'SENT'::public.chat_message_status NOT NULL,
    deleted_at timestamp(3) without time zone,
    deleted_by_id integer,
    chat_room_id integer NOT NULL,
    sender_id integer NOT NULL,
    reply_to_id integer,
    created timestamp(3) without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated timestamp(3) without time zone NOT NULL
);

--
-- Name: chat_message_edit_history; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE IF NOT EXISTS public.chat_message_edit_history (
    id integer NOT NULL,
    previous_content character varying(2000) NOT NULL,
    chat_message_id integer NOT NULL,
    edited_at timestamp(3) without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL
);

--
-- Name: chat_message_edit_history_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE IF NOT EXISTS public.chat_message_edit_history_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

--
-- Name: chat_message_edit_history_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.chat_message_edit_history_id_seq OWNED BY public.chat_message_edit_history.id;

--
-- Name: chat_message_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE IF NOT EXISTS public.chat_message_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

--
-- Name: chat_message_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.chat_message_id_seq OWNED BY public.chat_message.id;

--
-- Name: chat_message_reaction; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE IF NOT EXISTS public.chat_message_reaction (
    id integer NOT NULL,
    emoji character varying(10) NOT NULL,
    chat_message_id integer NOT NULL,
    user_id integer NOT NULL,
    created timestamp(3) without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated timestamp(3) without time zone NOT NULL
);

--
-- Name: chat_message_reaction_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE IF NOT EXISTS public.chat_message_reaction_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

--
-- Name: chat_message_reaction_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.chat_message_reaction_id_seq OWNED BY public.chat_message_reaction.id;

--
-- Name: chat_message_seen; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE IF NOT EXISTS public.chat_message_seen (
    id integer NOT NULL,
    chat_message_id integer NOT NULL,
    user_id integer NOT NULL,
    read_at timestamp(3) without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL
);

--
-- Name: chat_message_seen_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE IF NOT EXISTS public.chat_message_seen_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

--
-- Name: chat_message_seen_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.chat_message_seen_id_seq OWNED BY public.chat_message_seen.id;

--
-- Name: chat_room; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE IF NOT EXISTS public.chat_room (
    id integer NOT NULL,
    name character varying(107) NOT NULL,
    link character varying(17) NOT NULL,
    avatar character varying(1007) NOT NULL,
    type public.chat_type DEFAULT 'PRIVATE'::public.chat_type NOT NULL,
    last_message_time timestamp(3) without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    created timestamp(3) without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated timestamp(3) without time zone NOT NULL
);

--
-- Name: chat_room_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE IF NOT EXISTS public.chat_room_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

--
-- Name: chat_room_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.chat_room_id_seq OWNED BY public.chat_room.id;

--
-- Name: cron_state; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE IF NOT EXISTS public.cron_state (
    name character varying(64) NOT NULL,
    last_id bigint DEFAULT 0 NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);

--
-- Name: patch; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE IF NOT EXISTS public.patch (
    id integer NOT NULL,
    vndb_id character varying(107) NOT NULL,
    status integer DEFAULT 0 NOT NULL,
    download integer DEFAULT 0 NOT NULL,
    view integer DEFAULT 0 NOT NULL,
    type jsonb DEFAULT '[]'::jsonb,
    language jsonb DEFAULT '[]'::jsonb,
    platform jsonb DEFAULT '[]'::jsonb,
    user_id integer NOT NULL,
    created timestamp(3) without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated timestamp(3) without time zone NOT NULL,
    resource_update_time timestamp(3) without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    bid integer,
    favorite_count integer DEFAULT 0 NOT NULL,
    contribute_count integer DEFAULT 0 NOT NULL,
    comment_count integer DEFAULT 0 NOT NULL,
    resource_count integer DEFAULT 0 NOT NULL
);

--
-- Name: patch_comment; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE IF NOT EXISTS public.patch_comment (
    id integer NOT NULL,
    content character varying(10007) DEFAULT ''::character varying NOT NULL,
    edit text DEFAULT ''::text NOT NULL,
    parent_id integer,
    user_id integer NOT NULL,
    galgame_id integer CONSTRAINT patch_comment_patch_id_not_null NOT NULL,
    created timestamp(3) without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated timestamp(3) without time zone NOT NULL,
    like_count integer DEFAULT 0 NOT NULL
);

--
-- Name: patch_comment_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE IF NOT EXISTS public.patch_comment_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

--
-- Name: patch_comment_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.patch_comment_id_seq OWNED BY public.patch_comment.id;

--
-- Name: patch_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE IF NOT EXISTS public.patch_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

--
-- Name: patch_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.patch_id_seq OWNED BY public.patch.id;

--
-- Name: patch_link; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE IF NOT EXISTS public.patch_link (
    id integer NOT NULL,
    galgame_id integer CONSTRAINT patch_link_patch_id_not_null NOT NULL,
    name character varying(233) NOT NULL,
    url character varying(1007) NOT NULL,
    created timestamp(3) without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated timestamp(3) without time zone NOT NULL
);

--
-- Name: patch_link_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE IF NOT EXISTS public.patch_link_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

--
-- Name: patch_link_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.patch_link_id_seq OWNED BY public.patch_link.id;

--
-- Name: patch_resource; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE IF NOT EXISTS public.patch_resource (
    id integer NOT NULL,
    storage text NOT NULL,
    size character varying(107) DEFAULT ''::character varying NOT NULL,
    code character varying(1007) DEFAULT ''::character varying NOT NULL,
    password character varying(1007) DEFAULT ''::character varying NOT NULL,
    note character varying(10007) DEFAULT ''::character varying NOT NULL,
    blake3 text DEFAULT ''::text CONSTRAINT patch_resource_hash_not_null NOT NULL,
    content text DEFAULT ''::text NOT NULL,
    type jsonb DEFAULT '[]'::jsonb,
    language jsonb DEFAULT '[]'::jsonb,
    platform jsonb DEFAULT '[]'::jsonb,
    download integer DEFAULT 0 NOT NULL,
    status integer DEFAULT 0 NOT NULL,
    user_id integer NOT NULL,
    galgame_id integer CONSTRAINT patch_resource_patch_id_not_null NOT NULL,
    created timestamp(3) without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated timestamp(3) without time zone NOT NULL,
    model_name character varying(1007) DEFAULT ''::character varying NOT NULL,
    name character varying(300) DEFAULT ''::character varying NOT NULL,
    update_time timestamp(3) without time zone DEFAULT CURRENT_TIMESTAMP CONSTRAINT patch_resource_update_time_not_null1 NOT NULL,
    localization_group_name character varying(1007) DEFAULT ''::character varying NOT NULL,
    like_count integer DEFAULT 0 NOT NULL,
    s3_key character varying(2048) DEFAULT ''::character varying NOT NULL
);

--
-- Name: patch_resource_file_history; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE IF NOT EXISTS public.patch_resource_file_history (
    id bigint NOT NULL,
    resource_id integer NOT NULL,
    old_storage character varying(16) NOT NULL,
    old_s3_key character varying(2048) DEFAULT ''::character varying NOT NULL,
    old_blake3 character varying(128) DEFAULT ''::character varying NOT NULL,
    old_size character varying(107) DEFAULT ''::character varying NOT NULL,
    old_content text DEFAULT ''::text NOT NULL,
    reason character varying(500) DEFAULT ''::character varying NOT NULL,
    actor_id integer NOT NULL,
    actor_role integer DEFAULT 0 NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);

--
-- Name: patch_resource_file_history_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE IF NOT EXISTS public.patch_resource_file_history_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

--
-- Name: patch_resource_file_history_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.patch_resource_file_history_id_seq OWNED BY public.patch_resource_file_history.id;

--
-- Name: patch_resource_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE IF NOT EXISTS public.patch_resource_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

--
-- Name: patch_resource_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.patch_resource_id_seq OWNED BY public.patch_resource.id;

--
-- Name: user; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE IF NOT EXISTS public."user" (
    id integer NOT NULL,
    ip character varying(233) DEFAULT ''::character varying NOT NULL,
    moemoepoint integer DEFAULT 0 NOT NULL,
    daily_image_count integer DEFAULT 0 NOT NULL,
    daily_check_in integer DEFAULT 0 NOT NULL,
    daily_upload_size integer DEFAULT 0 NOT NULL,
    last_login_time text DEFAULT ''::text NOT NULL,
    created timestamp(3) without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated timestamp(3) without time zone NOT NULL,
    follower_count integer DEFAULT 0 NOT NULL,
    following_count integer DEFAULT 0 NOT NULL
);

--
-- Name: user_follow_relation; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE IF NOT EXISTS public.user_follow_relation (
    id integer NOT NULL,
    follower_id integer NOT NULL,
    following_id integer NOT NULL
);

--
-- Name: user_follow_relation_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE IF NOT EXISTS public.user_follow_relation_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

--
-- Name: user_follow_relation_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.user_follow_relation_id_seq OWNED BY public.user_follow_relation.id;

--
-- Name: user_message; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE IF NOT EXISTS public.user_message (
    id integer NOT NULL,
    type text NOT NULL,
    content character varying(10007) NOT NULL,
    status integer DEFAULT 0 NOT NULL,
    sender_id integer,
    recipient_id integer,
    created timestamp(3) without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated timestamp(3) without time zone NOT NULL,
    link character varying(1007) DEFAULT ''::character varying NOT NULL
);

--
-- Name: user_message_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE IF NOT EXISTS public.user_message_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

--
-- Name: user_message_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.user_message_id_seq OWNED BY public.user_message.id;

--
-- Name: user_patch_comment_like_relation; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE IF NOT EXISTS public.user_patch_comment_like_relation (
    id integer NOT NULL,
    user_id integer NOT NULL,
    comment_id integer NOT NULL,
    created timestamp(3) without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated timestamp(3) without time zone NOT NULL
);

--
-- Name: user_patch_comment_like_relation_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE IF NOT EXISTS public.user_patch_comment_like_relation_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

--
-- Name: user_patch_comment_like_relation_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.user_patch_comment_like_relation_id_seq OWNED BY public.user_patch_comment_like_relation.id;

--
-- Name: user_patch_contribute_relation; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE IF NOT EXISTS public.user_patch_contribute_relation (
    id integer NOT NULL,
    user_id integer NOT NULL,
    galgame_id integer CONSTRAINT user_patch_contribute_relation_patch_id_not_null NOT NULL,
    created timestamp(3) without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated timestamp(3) without time zone NOT NULL
);

--
-- Name: user_patch_contribute_relation_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE IF NOT EXISTS public.user_patch_contribute_relation_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

--
-- Name: user_patch_contribute_relation_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.user_patch_contribute_relation_id_seq OWNED BY public.user_patch_contribute_relation.id;

--
-- Name: user_patch_favorite_relation; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE IF NOT EXISTS public.user_patch_favorite_relation (
    id integer NOT NULL,
    user_id integer NOT NULL,
    galgame_id integer CONSTRAINT user_patch_favorite_relation_patch_id_not_null NOT NULL,
    created timestamp(3) without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated timestamp(3) without time zone NOT NULL
);

--
-- Name: user_patch_favorite_relation_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE IF NOT EXISTS public.user_patch_favorite_relation_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

--
-- Name: user_patch_favorite_relation_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.user_patch_favorite_relation_id_seq OWNED BY public.user_patch_favorite_relation.id;

--
-- Name: user_patch_resource_like_relation; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE IF NOT EXISTS public.user_patch_resource_like_relation (
    id integer NOT NULL,
    user_id integer NOT NULL,
    resource_id integer NOT NULL,
    created timestamp(3) without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated timestamp(3) without time zone NOT NULL
);

--
-- Name: user_patch_resource_like_relation_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE IF NOT EXISTS public.user_patch_resource_like_relation_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

--
-- Name: user_patch_resource_like_relation_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.user_patch_resource_like_relation_id_seq OWNED BY public.user_patch_resource_like_relation.id;

--
-- Name: wiki_message_processed; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE IF NOT EXISTS public.wiki_message_processed (
    message_id bigint NOT NULL,
    processed_at timestamp with time zone DEFAULT now() NOT NULL
);

--
-- Name: wiki_message_read_state; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE IF NOT EXISTS public.wiki_message_read_state (
    user_id integer NOT NULL,
    last_read_message_id bigint DEFAULT 0 NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);

--
-- Name: _migrations id; Type: DEFAULT; Schema: public; Owner: -
--

--
-- Name: admin_log id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.admin_log ALTER COLUMN id SET DEFAULT nextval('public.admin_log_id_seq'::regclass);

--
-- Name: chat_member id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.chat_member ALTER COLUMN id SET DEFAULT nextval('public.chat_member_id_seq'::regclass);

--
-- Name: chat_message id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.chat_message ALTER COLUMN id SET DEFAULT nextval('public.chat_message_id_seq'::regclass);

--
-- Name: chat_message_edit_history id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.chat_message_edit_history ALTER COLUMN id SET DEFAULT nextval('public.chat_message_edit_history_id_seq'::regclass);

--
-- Name: chat_message_reaction id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.chat_message_reaction ALTER COLUMN id SET DEFAULT nextval('public.chat_message_reaction_id_seq'::regclass);

--
-- Name: chat_message_seen id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.chat_message_seen ALTER COLUMN id SET DEFAULT nextval('public.chat_message_seen_id_seq'::regclass);

--
-- Name: chat_room id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.chat_room ALTER COLUMN id SET DEFAULT nextval('public.chat_room_id_seq'::regclass);

--
-- Name: patch id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.patch ALTER COLUMN id SET DEFAULT nextval('public.patch_id_seq'::regclass);

--
-- Name: patch_comment id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.patch_comment ALTER COLUMN id SET DEFAULT nextval('public.patch_comment_id_seq'::regclass);

--
-- Name: patch_link id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.patch_link ALTER COLUMN id SET DEFAULT nextval('public.patch_link_id_seq'::regclass);

--
-- Name: patch_resource id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.patch_resource ALTER COLUMN id SET DEFAULT nextval('public.patch_resource_id_seq'::regclass);

--
-- Name: patch_resource_file_history id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.patch_resource_file_history ALTER COLUMN id SET DEFAULT nextval('public.patch_resource_file_history_id_seq'::regclass);

--
-- Name: user_follow_relation id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_follow_relation ALTER COLUMN id SET DEFAULT nextval('public.user_follow_relation_id_seq'::regclass);

--
-- Name: user_message id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_message ALTER COLUMN id SET DEFAULT nextval('public.user_message_id_seq'::regclass);

--
-- Name: user_patch_comment_like_relation id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_patch_comment_like_relation ALTER COLUMN id SET DEFAULT nextval('public.user_patch_comment_like_relation_id_seq'::regclass);

--
-- Name: user_patch_contribute_relation id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_patch_contribute_relation ALTER COLUMN id SET DEFAULT nextval('public.user_patch_contribute_relation_id_seq'::regclass);

--
-- Name: user_patch_favorite_relation id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_patch_favorite_relation ALTER COLUMN id SET DEFAULT nextval('public.user_patch_favorite_relation_id_seq'::regclass);

--
-- Name: user_patch_resource_like_relation id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_patch_resource_like_relation ALTER COLUMN id SET DEFAULT nextval('public.user_patch_resource_like_relation_id_seq'::regclass);

--
-- Name: _migrations _migrations_name_key; Type: CONSTRAINT; Schema: public; Owner: -
--

--
-- Name: _migrations _migrations_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

--
-- Name: admin_log admin_log_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

DO $$ BEGIN
  ALTER TABLE ONLY public.admin_log
    ADD CONSTRAINT admin_log_pkey PRIMARY KEY (id);
EXCEPTION WHEN OTHERS THEN
  RAISE NOTICE 'baseline skip: admin_log_pkey (%) — %', SQLSTATE, SQLERRM;
END $$;

--
-- Name: chat_member chat_member_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

DO $$ BEGIN
  ALTER TABLE ONLY public.chat_member
    ADD CONSTRAINT chat_member_pkey PRIMARY KEY (id);
EXCEPTION WHEN OTHERS THEN
  RAISE NOTICE 'baseline skip: chat_member_pkey (%) — %', SQLSTATE, SQLERRM;
END $$;

--
-- Name: chat_message_edit_history chat_message_edit_history_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

DO $$ BEGIN
  ALTER TABLE ONLY public.chat_message_edit_history
    ADD CONSTRAINT chat_message_edit_history_pkey PRIMARY KEY (id);
EXCEPTION WHEN OTHERS THEN
  RAISE NOTICE 'baseline skip: chat_message_edit_history_pkey (%) — %', SQLSTATE, SQLERRM;
END $$;

--
-- Name: chat_message chat_message_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

DO $$ BEGIN
  ALTER TABLE ONLY public.chat_message
    ADD CONSTRAINT chat_message_pkey PRIMARY KEY (id);
EXCEPTION WHEN OTHERS THEN
  RAISE NOTICE 'baseline skip: chat_message_pkey (%) — %', SQLSTATE, SQLERRM;
END $$;

--
-- Name: chat_message_reaction chat_message_reaction_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

DO $$ BEGIN
  ALTER TABLE ONLY public.chat_message_reaction
    ADD CONSTRAINT chat_message_reaction_pkey PRIMARY KEY (id);
EXCEPTION WHEN OTHERS THEN
  RAISE NOTICE 'baseline skip: chat_message_reaction_pkey (%) — %', SQLSTATE, SQLERRM;
END $$;

--
-- Name: chat_message_seen chat_message_seen_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

DO $$ BEGIN
  ALTER TABLE ONLY public.chat_message_seen
    ADD CONSTRAINT chat_message_seen_pkey PRIMARY KEY (id);
EXCEPTION WHEN OTHERS THEN
  RAISE NOTICE 'baseline skip: chat_message_seen_pkey (%) — %', SQLSTATE, SQLERRM;
END $$;

--
-- Name: chat_room chat_room_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

DO $$ BEGIN
  ALTER TABLE ONLY public.chat_room
    ADD CONSTRAINT chat_room_pkey PRIMARY KEY (id);
EXCEPTION WHEN OTHERS THEN
  RAISE NOTICE 'baseline skip: chat_room_pkey (%) — %', SQLSTATE, SQLERRM;
END $$;

--
-- Name: cron_state cron_state_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

DO $$ BEGIN
  ALTER TABLE ONLY public.cron_state
    ADD CONSTRAINT cron_state_pkey PRIMARY KEY (name);
EXCEPTION WHEN OTHERS THEN
  RAISE NOTICE 'baseline skip: cron_state_pkey (%) — %', SQLSTATE, SQLERRM;
END $$;

--
-- Name: patch_comment patch_comment_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

DO $$ BEGIN
  ALTER TABLE ONLY public.patch_comment
    ADD CONSTRAINT patch_comment_pkey PRIMARY KEY (id);
EXCEPTION WHEN OTHERS THEN
  RAISE NOTICE 'baseline skip: patch_comment_pkey (%) — %', SQLSTATE, SQLERRM;
END $$;

--
-- Name: patch_link patch_link_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

DO $$ BEGIN
  ALTER TABLE ONLY public.patch_link
    ADD CONSTRAINT patch_link_pkey PRIMARY KEY (id);
EXCEPTION WHEN OTHERS THEN
  RAISE NOTICE 'baseline skip: patch_link_pkey (%) — %', SQLSTATE, SQLERRM;
END $$;

--
-- Name: patch patch_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

DO $$ BEGIN
  ALTER TABLE ONLY public.patch
    ADD CONSTRAINT patch_pkey PRIMARY KEY (id);
EXCEPTION WHEN OTHERS THEN
  RAISE NOTICE 'baseline skip: patch_pkey (%) — %', SQLSTATE, SQLERRM;
END $$;

--
-- Name: patch_resource_file_history patch_resource_file_history_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

DO $$ BEGIN
  ALTER TABLE ONLY public.patch_resource_file_history
    ADD CONSTRAINT patch_resource_file_history_pkey PRIMARY KEY (id);
EXCEPTION WHEN OTHERS THEN
  RAISE NOTICE 'baseline skip: patch_resource_file_history_pkey (%) — %', SQLSTATE, SQLERRM;
END $$;

--
-- Name: patch_resource patch_resource_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

DO $$ BEGIN
  ALTER TABLE ONLY public.patch_resource
    ADD CONSTRAINT patch_resource_pkey PRIMARY KEY (id);
EXCEPTION WHEN OTHERS THEN
  RAISE NOTICE 'baseline skip: patch_resource_pkey (%) — %', SQLSTATE, SQLERRM;
END $$;

--
-- Name: user_follow_relation user_follow_relation_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

DO $$ BEGIN
  ALTER TABLE ONLY public.user_follow_relation
    ADD CONSTRAINT user_follow_relation_pkey PRIMARY KEY (id);
EXCEPTION WHEN OTHERS THEN
  RAISE NOTICE 'baseline skip: user_follow_relation_pkey (%) — %', SQLSTATE, SQLERRM;
END $$;

--
-- Name: user_message user_message_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

DO $$ BEGIN
  ALTER TABLE ONLY public.user_message
    ADD CONSTRAINT user_message_pkey PRIMARY KEY (id);
EXCEPTION WHEN OTHERS THEN
  RAISE NOTICE 'baseline skip: user_message_pkey (%) — %', SQLSTATE, SQLERRM;
END $$;

--
-- Name: user_patch_comment_like_relation user_patch_comment_like_relation_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

DO $$ BEGIN
  ALTER TABLE ONLY public.user_patch_comment_like_relation
    ADD CONSTRAINT user_patch_comment_like_relation_pkey PRIMARY KEY (id);
EXCEPTION WHEN OTHERS THEN
  RAISE NOTICE 'baseline skip: user_patch_comment_like_relation_pkey (%) — %', SQLSTATE, SQLERRM;
END $$;

--
-- Name: user_patch_contribute_relation user_patch_contribute_relation_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

DO $$ BEGIN
  ALTER TABLE ONLY public.user_patch_contribute_relation
    ADD CONSTRAINT user_patch_contribute_relation_pkey PRIMARY KEY (id);
EXCEPTION WHEN OTHERS THEN
  RAISE NOTICE 'baseline skip: user_patch_contribute_relation_pkey (%) — %', SQLSTATE, SQLERRM;
END $$;

--
-- Name: user_patch_favorite_relation user_patch_favorite_relation_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

DO $$ BEGIN
  ALTER TABLE ONLY public.user_patch_favorite_relation
    ADD CONSTRAINT user_patch_favorite_relation_pkey PRIMARY KEY (id);
EXCEPTION WHEN OTHERS THEN
  RAISE NOTICE 'baseline skip: user_patch_favorite_relation_pkey (%) — %', SQLSTATE, SQLERRM;
END $$;

--
-- Name: user_patch_resource_like_relation user_patch_resource_like_relation_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

DO $$ BEGIN
  ALTER TABLE ONLY public.user_patch_resource_like_relation
    ADD CONSTRAINT user_patch_resource_like_relation_pkey PRIMARY KEY (id);
EXCEPTION WHEN OTHERS THEN
  RAISE NOTICE 'baseline skip: user_patch_resource_like_relation_pkey (%) — %', SQLSTATE, SQLERRM;
END $$;

--
-- Name: user user_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

DO $$ BEGIN
  ALTER TABLE ONLY public."user"
    ADD CONSTRAINT user_pkey PRIMARY KEY (id);
EXCEPTION WHEN OTHERS THEN
  RAISE NOTICE 'baseline skip: user_pkey (%) — %', SQLSTATE, SQLERRM;
END $$;

--
-- Name: wiki_message_processed wiki_message_processed_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

DO $$ BEGIN
  ALTER TABLE ONLY public.wiki_message_processed
    ADD CONSTRAINT wiki_message_processed_pkey PRIMARY KEY (message_id);
EXCEPTION WHEN OTHERS THEN
  RAISE NOTICE 'baseline skip: wiki_message_processed_pkey (%) — %', SQLSTATE, SQLERRM;
END $$;

--
-- Name: wiki_message_read_state wiki_message_read_state_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

DO $$ BEGIN
  ALTER TABLE ONLY public.wiki_message_read_state
    ADD CONSTRAINT wiki_message_read_state_pkey PRIMARY KEY (user_id);
EXCEPTION WHEN OTHERS THEN
  RAISE NOTICE 'baseline skip: wiki_message_read_state_pkey (%) — %', SQLSTATE, SQLERRM;
END $$;

--
-- Name: chat_member_user_id_chat_room_id_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX IF NOT EXISTS chat_member_user_id_chat_room_id_key ON public.chat_member USING btree (user_id, chat_room_id);

--
-- Name: chat_message_chat_room_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX IF NOT EXISTS chat_message_chat_room_id_idx ON public.chat_message USING btree (chat_room_id);

--
-- Name: chat_message_edit_history_chat_message_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX IF NOT EXISTS chat_message_edit_history_chat_message_id_idx ON public.chat_message_edit_history USING btree (chat_message_id);

--
-- Name: chat_message_reaction_user_id_chat_message_id_emoji_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX IF NOT EXISTS chat_message_reaction_user_id_chat_message_id_emoji_key ON public.chat_message_reaction USING btree (user_id, chat_message_id, emoji);

--
-- Name: chat_message_seen_user_id_chat_message_id_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX IF NOT EXISTS chat_message_seen_user_id_chat_message_id_key ON public.chat_message_seen USING btree (user_id, chat_message_id);

--
-- Name: chat_room_link_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX IF NOT EXISTS chat_room_link_key ON public.chat_room USING btree (link);

--
-- Name: chat_room_type_name_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX IF NOT EXISTS chat_room_type_name_idx ON public.chat_room USING btree (type, name);

--
-- Name: idx_patch_resource_s3_key; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX IF NOT EXISTS idx_patch_resource_s3_key ON public.patch_resource USING btree (s3_key) WHERE ((s3_key)::text <> ''::text);

--
-- Name: idx_prfh_resource; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX IF NOT EXISTS idx_prfh_resource ON public.patch_resource_file_history USING btree (resource_id, created_at DESC);

--
-- Name: patch_bid_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX IF NOT EXISTS patch_bid_key ON public.patch USING btree (bid);

--
-- Name: patch_link_name_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX IF NOT EXISTS patch_link_name_idx ON public.patch_link USING btree (name);

--
-- Name: patch_link_patch_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX IF NOT EXISTS patch_link_patch_id_idx ON public.patch_link USING btree (galgame_id);

--
-- Name: patch_link_patch_id_name_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX IF NOT EXISTS patch_link_patch_id_name_key ON public.patch_link USING btree (galgame_id, name);

--
-- Name: patch_vndb_id_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX IF NOT EXISTS patch_vndb_id_key ON public.patch USING btree (vndb_id);

--
-- Name: user_follow_relation_follower_id_following_id_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX IF NOT EXISTS user_follow_relation_follower_id_following_id_key ON public.user_follow_relation USING btree (follower_id, following_id);

--
-- Name: user_patch_comment_like_relation_user_id_comment_id_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX IF NOT EXISTS user_patch_comment_like_relation_user_id_comment_id_key ON public.user_patch_comment_like_relation USING btree (user_id, comment_id);

--
-- Name: user_patch_contribute_relation_user_id_patch_id_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX IF NOT EXISTS user_patch_contribute_relation_user_id_patch_id_key ON public.user_patch_contribute_relation USING btree (user_id, galgame_id);

--
-- Name: user_patch_favorite_relation_user_id_patch_id_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX IF NOT EXISTS user_patch_favorite_relation_user_id_patch_id_key ON public.user_patch_favorite_relation USING btree (user_id, galgame_id);

--
-- Name: user_patch_resource_like_relation_user_id_resource_id_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX IF NOT EXISTS user_patch_resource_like_relation_user_id_resource_id_key ON public.user_patch_resource_like_relation USING btree (user_id, resource_id);

--
-- Name: admin_log admin_log_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

DO $$ BEGIN
  ALTER TABLE ONLY public.admin_log
    ADD CONSTRAINT admin_log_user_id_fkey FOREIGN KEY (user_id) REFERENCES public."user"(id) ON DELETE CASCADE;
EXCEPTION WHEN OTHERS THEN
  RAISE NOTICE 'baseline skip: admin_log_user_id_fkey (%) — %', SQLSTATE, SQLERRM;
END $$;

--
-- Name: chat_member chat_member_chat_room_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

DO $$ BEGIN
  ALTER TABLE ONLY public.chat_member
    ADD CONSTRAINT chat_member_chat_room_id_fkey FOREIGN KEY (chat_room_id) REFERENCES public.chat_room(id) ON UPDATE CASCADE ON DELETE CASCADE;
EXCEPTION WHEN OTHERS THEN
  RAISE NOTICE 'baseline skip: chat_member_chat_room_id_fkey (%) — %', SQLSTATE, SQLERRM;
END $$;

--
-- Name: chat_member chat_member_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

DO $$ BEGIN
  ALTER TABLE ONLY public.chat_member
    ADD CONSTRAINT chat_member_user_id_fkey FOREIGN KEY (user_id) REFERENCES public."user"(id) ON UPDATE CASCADE ON DELETE CASCADE;
EXCEPTION WHEN OTHERS THEN
  RAISE NOTICE 'baseline skip: chat_member_user_id_fkey (%) — %', SQLSTATE, SQLERRM;
END $$;

--
-- Name: chat_message chat_message_chat_room_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

DO $$ BEGIN
  ALTER TABLE ONLY public.chat_message
    ADD CONSTRAINT chat_message_chat_room_id_fkey FOREIGN KEY (chat_room_id) REFERENCES public.chat_room(id) ON UPDATE CASCADE ON DELETE CASCADE;
EXCEPTION WHEN OTHERS THEN
  RAISE NOTICE 'baseline skip: chat_message_chat_room_id_fkey (%) — %', SQLSTATE, SQLERRM;
END $$;

--
-- Name: chat_message chat_message_deleted_by_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

DO $$ BEGIN
  ALTER TABLE ONLY public.chat_message
    ADD CONSTRAINT chat_message_deleted_by_id_fkey FOREIGN KEY (deleted_by_id) REFERENCES public."user"(id) ON UPDATE CASCADE ON DELETE SET NULL;
EXCEPTION WHEN OTHERS THEN
  RAISE NOTICE 'baseline skip: chat_message_deleted_by_id_fkey (%) — %', SQLSTATE, SQLERRM;
END $$;

--
-- Name: chat_message_edit_history chat_message_edit_history_chat_message_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

DO $$ BEGIN
  ALTER TABLE ONLY public.chat_message_edit_history
    ADD CONSTRAINT chat_message_edit_history_chat_message_id_fkey FOREIGN KEY (chat_message_id) REFERENCES public.chat_message(id) ON UPDATE CASCADE ON DELETE CASCADE;
EXCEPTION WHEN OTHERS THEN
  RAISE NOTICE 'baseline skip: chat_message_edit_history_chat_message_id_fkey (%) — %', SQLSTATE, SQLERRM;
END $$;

--
-- Name: chat_message_reaction chat_message_reaction_chat_message_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

DO $$ BEGIN
  ALTER TABLE ONLY public.chat_message_reaction
    ADD CONSTRAINT chat_message_reaction_chat_message_id_fkey FOREIGN KEY (chat_message_id) REFERENCES public.chat_message(id) ON UPDATE CASCADE ON DELETE CASCADE;
EXCEPTION WHEN OTHERS THEN
  RAISE NOTICE 'baseline skip: chat_message_reaction_chat_message_id_fkey (%) — %', SQLSTATE, SQLERRM;
END $$;

--
-- Name: chat_message_reaction chat_message_reaction_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

DO $$ BEGIN
  ALTER TABLE ONLY public.chat_message_reaction
    ADD CONSTRAINT chat_message_reaction_user_id_fkey FOREIGN KEY (user_id) REFERENCES public."user"(id) ON UPDATE CASCADE ON DELETE CASCADE;
EXCEPTION WHEN OTHERS THEN
  RAISE NOTICE 'baseline skip: chat_message_reaction_user_id_fkey (%) — %', SQLSTATE, SQLERRM;
END $$;

--
-- Name: chat_message chat_message_reply_to_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

DO $$ BEGIN
  ALTER TABLE ONLY public.chat_message
    ADD CONSTRAINT chat_message_reply_to_id_fkey FOREIGN KEY (reply_to_id) REFERENCES public.chat_message(id) ON DELETE SET NULL;
EXCEPTION WHEN OTHERS THEN
  RAISE NOTICE 'baseline skip: chat_message_reply_to_id_fkey (%) — %', SQLSTATE, SQLERRM;
END $$;

--
-- Name: chat_message_seen chat_message_seen_chat_message_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

DO $$ BEGIN
  ALTER TABLE ONLY public.chat_message_seen
    ADD CONSTRAINT chat_message_seen_chat_message_id_fkey FOREIGN KEY (chat_message_id) REFERENCES public.chat_message(id) ON UPDATE CASCADE ON DELETE CASCADE;
EXCEPTION WHEN OTHERS THEN
  RAISE NOTICE 'baseline skip: chat_message_seen_chat_message_id_fkey (%) — %', SQLSTATE, SQLERRM;
END $$;

--
-- Name: chat_message_seen chat_message_seen_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

DO $$ BEGIN
  ALTER TABLE ONLY public.chat_message_seen
    ADD CONSTRAINT chat_message_seen_user_id_fkey FOREIGN KEY (user_id) REFERENCES public."user"(id) ON UPDATE CASCADE ON DELETE CASCADE;
EXCEPTION WHEN OTHERS THEN
  RAISE NOTICE 'baseline skip: chat_message_seen_user_id_fkey (%) — %', SQLSTATE, SQLERRM;
END $$;

--
-- Name: chat_message chat_message_sender_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

DO $$ BEGIN
  ALTER TABLE ONLY public.chat_message
    ADD CONSTRAINT chat_message_sender_id_fkey FOREIGN KEY (sender_id) REFERENCES public."user"(id) ON UPDATE CASCADE ON DELETE CASCADE;
EXCEPTION WHEN OTHERS THEN
  RAISE NOTICE 'baseline skip: chat_message_sender_id_fkey (%) — %', SQLSTATE, SQLERRM;
END $$;

--
-- Name: patch_comment patch_comment_parent_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

DO $$ BEGIN
  ALTER TABLE ONLY public.patch_comment
    ADD CONSTRAINT patch_comment_parent_id_fkey FOREIGN KEY (parent_id) REFERENCES public.patch_comment(id) ON DELETE CASCADE;
EXCEPTION WHEN OTHERS THEN
  RAISE NOTICE 'baseline skip: patch_comment_parent_id_fkey (%) — %', SQLSTATE, SQLERRM;
END $$;

--
-- Name: patch_comment patch_comment_patch_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

DO $$ BEGIN
  ALTER TABLE ONLY public.patch_comment
    ADD CONSTRAINT patch_comment_patch_id_fkey FOREIGN KEY (galgame_id) REFERENCES public.patch(id) ON DELETE CASCADE;
EXCEPTION WHEN OTHERS THEN
  RAISE NOTICE 'baseline skip: patch_comment_patch_id_fkey (%) — %', SQLSTATE, SQLERRM;
END $$;

--
-- Name: patch_comment patch_comment_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

DO $$ BEGIN
  ALTER TABLE ONLY public.patch_comment
    ADD CONSTRAINT patch_comment_user_id_fkey FOREIGN KEY (user_id) REFERENCES public."user"(id) ON DELETE CASCADE;
EXCEPTION WHEN OTHERS THEN
  RAISE NOTICE 'baseline skip: patch_comment_user_id_fkey (%) — %', SQLSTATE, SQLERRM;
END $$;

--
-- Name: patch_link patch_link_patch_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

DO $$ BEGIN
  ALTER TABLE ONLY public.patch_link
    ADD CONSTRAINT patch_link_patch_id_fkey FOREIGN KEY (galgame_id) REFERENCES public.patch(id) ON DELETE CASCADE;
EXCEPTION WHEN OTHERS THEN
  RAISE NOTICE 'baseline skip: patch_link_patch_id_fkey (%) — %', SQLSTATE, SQLERRM;
END $$;

--
-- Name: patch_resource_file_history patch_resource_file_history_resource_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

DO $$ BEGIN
  ALTER TABLE ONLY public.patch_resource_file_history
    ADD CONSTRAINT patch_resource_file_history_resource_id_fkey FOREIGN KEY (resource_id) REFERENCES public.patch_resource(id) ON DELETE CASCADE;
EXCEPTION WHEN OTHERS THEN
  RAISE NOTICE 'baseline skip: patch_resource_file_history_resource_id_fkey (%) — %', SQLSTATE, SQLERRM;
END $$;

--
-- Name: patch_resource patch_resource_patch_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

DO $$ BEGIN
  ALTER TABLE ONLY public.patch_resource
    ADD CONSTRAINT patch_resource_patch_id_fkey FOREIGN KEY (galgame_id) REFERENCES public.patch(id) ON DELETE CASCADE;
EXCEPTION WHEN OTHERS THEN
  RAISE NOTICE 'baseline skip: patch_resource_patch_id_fkey (%) — %', SQLSTATE, SQLERRM;
END $$;

--
-- Name: patch_resource patch_resource_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

DO $$ BEGIN
  ALTER TABLE ONLY public.patch_resource
    ADD CONSTRAINT patch_resource_user_id_fkey FOREIGN KEY (user_id) REFERENCES public."user"(id) ON DELETE CASCADE;
EXCEPTION WHEN OTHERS THEN
  RAISE NOTICE 'baseline skip: patch_resource_user_id_fkey (%) — %', SQLSTATE, SQLERRM;
END $$;

--
-- Name: patch patch_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

DO $$ BEGIN
  ALTER TABLE ONLY public.patch
    ADD CONSTRAINT patch_user_id_fkey FOREIGN KEY (user_id) REFERENCES public."user"(id) ON UPDATE CASCADE ON DELETE RESTRICT;
EXCEPTION WHEN OTHERS THEN
  RAISE NOTICE 'baseline skip: patch_user_id_fkey (%) — %', SQLSTATE, SQLERRM;
END $$;

--
-- Name: user_follow_relation user_follow_relation_follower_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

DO $$ BEGIN
  ALTER TABLE ONLY public.user_follow_relation
    ADD CONSTRAINT user_follow_relation_follower_id_fkey FOREIGN KEY (follower_id) REFERENCES public."user"(id) ON DELETE CASCADE;
EXCEPTION WHEN OTHERS THEN
  RAISE NOTICE 'baseline skip: user_follow_relation_follower_id_fkey (%) — %', SQLSTATE, SQLERRM;
END $$;

--
-- Name: user_follow_relation user_follow_relation_following_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

DO $$ BEGIN
  ALTER TABLE ONLY public.user_follow_relation
    ADD CONSTRAINT user_follow_relation_following_id_fkey FOREIGN KEY (following_id) REFERENCES public."user"(id) ON UPDATE CASCADE ON DELETE RESTRICT;
EXCEPTION WHEN OTHERS THEN
  RAISE NOTICE 'baseline skip: user_follow_relation_following_id_fkey (%) — %', SQLSTATE, SQLERRM;
END $$;

--
-- Name: user_message user_message_recipient_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

DO $$ BEGIN
  ALTER TABLE ONLY public.user_message
    ADD CONSTRAINT user_message_recipient_id_fkey FOREIGN KEY (recipient_id) REFERENCES public."user"(id) ON DELETE CASCADE;
EXCEPTION WHEN OTHERS THEN
  RAISE NOTICE 'baseline skip: user_message_recipient_id_fkey (%) — %', SQLSTATE, SQLERRM;
END $$;

--
-- Name: user_message user_message_sender_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

DO $$ BEGIN
  ALTER TABLE ONLY public.user_message
    ADD CONSTRAINT user_message_sender_id_fkey FOREIGN KEY (sender_id) REFERENCES public."user"(id) ON DELETE CASCADE;
EXCEPTION WHEN OTHERS THEN
  RAISE NOTICE 'baseline skip: user_message_sender_id_fkey (%) — %', SQLSTATE, SQLERRM;
END $$;

--
-- Name: user_patch_comment_like_relation user_patch_comment_like_relation_comment_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

DO $$ BEGIN
  ALTER TABLE ONLY public.user_patch_comment_like_relation
    ADD CONSTRAINT user_patch_comment_like_relation_comment_id_fkey FOREIGN KEY (comment_id) REFERENCES public.patch_comment(id) ON DELETE CASCADE;
EXCEPTION WHEN OTHERS THEN
  RAISE NOTICE 'baseline skip: user_patch_comment_like_relation_comment_id_fkey (%) — %', SQLSTATE, SQLERRM;
END $$;

--
-- Name: user_patch_comment_like_relation user_patch_comment_like_relation_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

DO $$ BEGIN
  ALTER TABLE ONLY public.user_patch_comment_like_relation
    ADD CONSTRAINT user_patch_comment_like_relation_user_id_fkey FOREIGN KEY (user_id) REFERENCES public."user"(id) ON DELETE CASCADE;
EXCEPTION WHEN OTHERS THEN
  RAISE NOTICE 'baseline skip: user_patch_comment_like_relation_user_id_fkey (%) — %', SQLSTATE, SQLERRM;
END $$;

--
-- Name: user_patch_contribute_relation user_patch_contribute_relation_patch_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

DO $$ BEGIN
  ALTER TABLE ONLY public.user_patch_contribute_relation
    ADD CONSTRAINT user_patch_contribute_relation_patch_id_fkey FOREIGN KEY (galgame_id) REFERENCES public.patch(id) ON DELETE CASCADE;
EXCEPTION WHEN OTHERS THEN
  RAISE NOTICE 'baseline skip: user_patch_contribute_relation_patch_id_fkey (%) — %', SQLSTATE, SQLERRM;
END $$;

--
-- Name: user_patch_contribute_relation user_patch_contribute_relation_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

DO $$ BEGIN
  ALTER TABLE ONLY public.user_patch_contribute_relation
    ADD CONSTRAINT user_patch_contribute_relation_user_id_fkey FOREIGN KEY (user_id) REFERENCES public."user"(id) ON DELETE CASCADE;
EXCEPTION WHEN OTHERS THEN
  RAISE NOTICE 'baseline skip: user_patch_contribute_relation_user_id_fkey (%) — %', SQLSTATE, SQLERRM;
END $$;

--
-- Name: user_patch_favorite_relation user_patch_favorite_relation_patch_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

DO $$ BEGIN
  ALTER TABLE ONLY public.user_patch_favorite_relation
    ADD CONSTRAINT user_patch_favorite_relation_patch_id_fkey FOREIGN KEY (galgame_id) REFERENCES public.patch(id) ON DELETE CASCADE;
EXCEPTION WHEN OTHERS THEN
  RAISE NOTICE 'baseline skip: user_patch_favorite_relation_patch_id_fkey (%) — %', SQLSTATE, SQLERRM;
END $$;

--
-- Name: user_patch_favorite_relation user_patch_favorite_relation_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

DO $$ BEGIN
  ALTER TABLE ONLY public.user_patch_favorite_relation
    ADD CONSTRAINT user_patch_favorite_relation_user_id_fkey FOREIGN KEY (user_id) REFERENCES public."user"(id) ON DELETE CASCADE;
EXCEPTION WHEN OTHERS THEN
  RAISE NOTICE 'baseline skip: user_patch_favorite_relation_user_id_fkey (%) — %', SQLSTATE, SQLERRM;
END $$;

--
-- Name: user_patch_resource_like_relation user_patch_resource_like_relation_resource_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

DO $$ BEGIN
  ALTER TABLE ONLY public.user_patch_resource_like_relation
    ADD CONSTRAINT user_patch_resource_like_relation_resource_id_fkey FOREIGN KEY (resource_id) REFERENCES public.patch_resource(id) ON DELETE CASCADE;
EXCEPTION WHEN OTHERS THEN
  RAISE NOTICE 'baseline skip: user_patch_resource_like_relation_resource_id_fkey (%) — %', SQLSTATE, SQLERRM;
END $$;

--
-- Name: user_patch_resource_like_relation user_patch_resource_like_relation_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

DO $$ BEGIN
  ALTER TABLE ONLY public.user_patch_resource_like_relation
    ADD CONSTRAINT user_patch_resource_like_relation_user_id_fkey FOREIGN KEY (user_id) REFERENCES public."user"(id) ON DELETE CASCADE;
EXCEPTION WHEN OTHERS THEN
  RAISE NOTICE 'baseline skip: user_patch_resource_like_relation_user_id_fkey (%) — %', SQLSTATE, SQLERRM;
END $$;

--
-- PostgreSQL database dump complete
--

