-- Really crude first data model. This is mostly for importing and cleaning up the information
-- we got. For the first round, just a simple flat table.
create table component (
       id    int       constraint pk_component primary key,
       category varchar(40),    -- should be some foreign key
       value varchar(80),       -- identifying the component value
       description text,        -- additional information
       notes text,              -- user notes, can contain hashtags.
       datasheet_url text,      -- data sheet URL if available
       vendor varchar(30),      -- should be foreign key
       auto_notes text,         -- auto generated notes, might help in search (including hashtags)
       footprint varchar(30),
       quantity varchar(5),	-- Initially text to allow freeform <50 or so. TODO: better.

       created timestamp,
       updated timestamp

       -- also, we need the following eventually
       -- labeltext, drawer-type, location. Several of these should have foreign keys.
);
