# runs all transforms needed for a fresh content port

sh scripts/_temp_rename.sh pages/docs;
sh scripts/_temp_rename.sh pages/intro;
sh scripts/_temp_rename.sh pages/api-docs;
node scripts/_temp_fix_unclosed_tags.js;
node scripts/_temp_fix_partials.js;
