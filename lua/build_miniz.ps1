gcc -c lminiz.c -IC:\lua\include -lC:\lua\liblua.a
gcc -shared lminiz.o -o miniz.dll C:\lua\lua53.dll