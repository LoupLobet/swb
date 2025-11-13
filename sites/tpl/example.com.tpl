<h1>
%{
	echo $page_name
}%
</h1>

Built with <code>swb</code>

%{
	$builder $src_path
}%

Templates can use config defined variables:
%{
	echo $test
}%

